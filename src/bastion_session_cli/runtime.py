"""Runtime helper functions for the CLI."""

from __future__ import annotations

import subprocess
from dataclasses import dataclass
from pathlib import Path

from rich.console import Console
from rich.table import Table

from .config import Config, StateStore
from .oci_client import BastionClient, TargetDetails, build_config
from .session import SessionCache
from .ssh_config import write_fragment
from .terraform import read_outputs, terraform_output_raw

console = Console()


@dataclass
class SessionMetadata:
    bastion_id: str
    instance_id: str
    private_ip: str
    bastion_host: str | None


def load_target_details(config: Config) -> SessionMetadata:
    outputs_path = config.terraform_outputs_path
    if outputs_path and outputs_path.exists():
        outputs = read_outputs(outputs_path)
    else:
        outputs = {
            "bastion_id": terraform_output_raw("bastion_id"),
            "instance_id": terraform_output_raw("instance_id"),
            "private_ip": terraform_output_raw("private_ip"),
        }
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
    host_entry = (
        f"Host {config.profile}-bastion\n"
        f"  HostName {metadata.private_ip}\n"
        f"  ProxyCommand oci bastion session connect --session-id {session_id}\n"
        f"  User {config.target_user}"
    )
    write_fragment(config.ssh_include_path, [host_entry])


def refresh_session(config: Config) -> None:
    state_store = StateStore(config.session_state_path)
    cache = SessionCache(state_store)

    metadata = load_target_details(config)

    if not config.ssh_public_key:
        raise RuntimeError("SSH_PUBLIC_KEY environment variable or config required")

    client = BastionClient(build_config(config.profile, config.region, config.auth_method))
    target = TargetDetails(
        bastion_id=metadata.bastion_id,
        instance_id=metadata.instance_id,
        private_ip=metadata.private_ip,
        target_user=config.target_user,
        public_key_path=str(config.ssh_public_key),
    )

    session = client.create_session(target)
    cache.set(session)
    ensure_includes(config)
    update_ssh_fragment(config, metadata, session.id)
    console.print(f"[green]Created session {session.id}, expires at {session.time_expires.isoformat()}")


def session_status(config: Config) -> None:
    cache = SessionCache(StateStore(config.session_state_path))
    session = cache.get()
    if not session:
        console.print("[yellow]No cached session. Run refresh first.")
        raise SystemExit(1)

    table = Table(title="Bastion Session Status")
    table.add_column("Field")
    table.add_column("Value")
    table.add_row("Session ID", session.id)
    table.add_row("Lifecycle", session.lifecycle_state)
    table.add_row("Expires", session.time_expires.isoformat())
    table.add_row("Expires In", str(session.expires_in))
    console.print(table)


def watch(config: Config, interval_seconds: int = 300) -> None:
    while True:
        try:
            refresh_session(config)
        except Exception as exc:
            console.print(f"[red]Failed to refresh session: {exc}")
        console.print(f"Sleeping for {interval_seconds} seconds")
        subprocess.run(["sleep", str(interval_seconds)], check=True)
