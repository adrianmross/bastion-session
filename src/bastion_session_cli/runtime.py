"""Runtime helper functions for the bastion-session CLI."""

from __future__ import annotations

import json
import time
from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path

import yaml
from rich.console import Console
from rich.table import Table

from .config import Config, StateStore
from .oci_client import BastionClient, TargetDetails
from .session import BastionSession, SessionCache
from .ssh_config import write_fragment
from .terraform import read_outputs


ACTIVE_POLL_INTERVAL_SECONDS = 5.0
ACTIVE_WAIT_TIMEOUT = timedelta(minutes=2)
DEFAULT_WATCH_INTERVAL_SECONDS = 300
MIN_AUTO_REFRESH_SECONDS = 30
AUTO_REFRESH_MARGIN = ACTIVE_WAIT_TIMEOUT + timedelta(seconds=30)

console = Console()


@dataclass
class SessionMetadata:
    bastion_id: str
    instance_id: str
    private_ip: str
    bastion_host: str | None


def _resolve_outputs_path(config: Config) -> Path | None:
    candidates: list[Path] = []
    seen: set[Path] = set()

    def add_candidate(path: Path | None) -> None:
        if path and path not in seen:
            candidates.append(path)
            seen.add(path)

    add_candidate(config.terraform_outputs_path if config.terraform_outputs_path else None)

    cwd = Path.cwd()
    search_roots = [cwd, *cwd.parents]
    filenames = ["terraform.tfstate", "terraform.tfstate.json", "outputs.json"]
    for root in search_roots:
        for name in filenames:
            add_candidate(root / name)

    repo_parents = list(Path(__file__).resolve().parents)
    if len(repo_parents) >= 5:
        repo_root = repo_parents[4]
        for name in ("terraform.tfstate", "outputs.json"):
            add_candidate(repo_root / name)

    for candidate in candidates:
        if candidate.exists():
            return candidate

    return None


def load_target_details(config: Config) -> SessionMetadata:
    outputs_path = _resolve_outputs_path(config)
    if not outputs_path:
        raise RuntimeError(
            "Unable to locate Terraform outputs; set TERRAFORM_OUTPUTS or provide terraform.tfstate"
        )

    outputs = read_outputs(outputs_path)

    missing = [key for key in ("bastion_id", "instance_id", "private_ip") if key not in outputs]
    if missing:
        raise RuntimeError(f"Missing outputs in {outputs_path}: {', '.join(missing)}")

    return SessionMetadata(
        bastion_id=outputs["bastion_id"],
        instance_id=outputs["instance_id"],
        private_ip=outputs["private_ip"],
        bastion_host=outputs.get("bastion_public_ip"),
    )


def ensure_includes(config: Config) -> None:
    config.ssh_include_path.parent.mkdir(parents=True, exist_ok=True)
    main_config = Path.home() / ".ssh" / "config"
    if not main_config.exists():
        return

    include_line = f"Include {config.ssh_include_path}"

    lines = main_config.read_text().splitlines()
    if include_line not in lines:
        lines.append(include_line)
        main_config.write_text("\n".join(lines) + "\n")
        console.print(f"[green]Added include line to {main_config}")


def update_ssh_fragment(config: Config, metadata: SessionMetadata, session_id: str) -> None:
    private_key = config.ssh_private_key
    if not private_key and config.ssh_public_key and str(config.ssh_public_key).endswith(".pub"):
        candidate = Path(str(config.ssh_public_key)[:-4])
        if candidate.exists():
            private_key = candidate

    host_entry_lines = [
        f"Host {config.profile}-bastion",
        f"  HostName host.bastion.{config.region}.oci.oraclecloud.com",
        "  Port 22",
        f"  User {session_id}",
    ]

    if private_key:
        host_entry_lines.append(f"  IdentityFile {private_key}")

    host_entry_lines.extend([
        "  IdentitiesOnly yes",
        "  IdentityAgent none",
    ])

    host_entry = "\n".join(host_entry_lines)
    write_fragment(config.ssh_include_path, [host_entry])


def _wait_for_active(
    client: BastionClient,
    session_id: str,
    timeout: timedelta,
    poll_interval: float,
) -> BastionSession:
    deadline = time.monotonic() + timeout.total_seconds()
    while True:
        session = client.get_session(session_id)
        if session.lifecycle_state.upper() == "ACTIVE":
            return session
        if time.monotonic() >= deadline:
            raise RuntimeError(
                f"Session {session_id} did not reach ACTIVE state (last state: {session.lifecycle_state})"
            )
        time.sleep(poll_interval)


def refresh_session(config: Config) -> BastionSession:
    state_store = StateStore(config.session_state_path)
    cache = SessionCache(state_store)

    metadata = load_target_details(config)

    if not config.ssh_public_key:
        raise RuntimeError("SSH_PUBLIC_KEY environment variable or config required")

    client = BastionClient(config.profile, config.region, config.auth_method)
    target = TargetDetails(
        bastion_id=metadata.bastion_id,
        instance_id=metadata.instance_id,
        private_ip=metadata.private_ip,
        target_user=config.target_user,
        public_key_path=str(config.ssh_public_key),
    )

    created_session = client.create_session(target)
    session = _wait_for_active(
        client,
        created_session.id,
        timeout=ACTIVE_WAIT_TIMEOUT,
        poll_interval=ACTIVE_POLL_INTERVAL_SECONDS,
    )
    cache.set(session)
    ensure_includes(config)
    update_ssh_fragment(config, metadata, session.id)
    console.print(f"[green]Created session {session.id}, expires at {session.time_expires.isoformat()}")
    return session


def _build_status_data(session: BastionSession) -> dict[str, str]:
    return {
        "session_id": session.id,
        "lifecycle": session.lifecycle_state,
        "expires": session.time_expires.isoformat(),
        "expires_in": str(session.expires_in),
    }


def session_status(config: Config, output_format: str = "table") -> None:
    cache = SessionCache(StateStore(config.session_state_path))
    cached_session = cache.get()
    if not cached_session:
        console.print("[yellow]No cached session. Run refresh first.")
        raise SystemExit(1)

    client = BastionClient(config.profile, config.region, config.auth_method)
    try:
        session = client.get_session(cached_session.id)
        cache.set(session)
    except Exception as exc:  # pragma: no cover - fallback path
        console.print(f"[yellow]Using cached session; failed to fetch live status: {exc}")
        session = cached_session

    status_data = _build_status_data(session)

    fmt = (output_format or "table").lower()
    if fmt == "table":
        table = Table(title="Bastion Session Status")
        table.add_column("Field")
        table.add_column("Value")
        table.add_row("Session ID", status_data["session_id"])
        table.add_row("Lifecycle", status_data["lifecycle"])
        table.add_row("Expires", status_data["expires"])
        table.add_row("Expires In", status_data["expires_in"])
        console.print(table)
    elif fmt == "json":
        status_json = json.dumps(status_data)
        console.file.write(status_json + "\n")
        console.file.flush()
    elif fmt == "yaml":
        console.print(yaml.safe_dump(status_data, sort_keys=False))
    else:  # pragma: no cover - guarded by CLI choices
        raise ValueError(f"Unsupported output format: {output_format}")


def _auto_refresh_interval(session: BastionSession) -> int:
    expires_in_seconds = max(session.expires_in.total_seconds(), 0)
    margin_seconds = AUTO_REFRESH_MARGIN.total_seconds()
    interval = expires_in_seconds - margin_seconds
    if interval < MIN_AUTO_REFRESH_SECONDS:
        interval = MIN_AUTO_REFRESH_SECONDS
    return int(interval)


def watch(config: Config, interval_seconds: int | None = None) -> None:
    explicit_interval = interval_seconds if interval_seconds and interval_seconds > 0 else None
    sleep_seconds = explicit_interval or DEFAULT_WATCH_INTERVAL_SECONDS
    while True:
        try:
            session = refresh_session(config)
            if explicit_interval is None:
                sleep_seconds = _auto_refresh_interval(session)
        except Exception as exc:  # pragma: no cover - background loop
            console.print(f"[red]Failed to refresh session: {exc}")
            sleep_seconds = explicit_interval or DEFAULT_WATCH_INTERVAL_SECONDS
        console.print(f"Sleeping for {sleep_seconds} seconds")
        time.sleep(sleep_seconds)
