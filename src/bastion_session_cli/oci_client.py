"""OCI Bastion helpers implemented via OCI CLI subprocess calls."""

from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass
from datetime import timedelta
from typing import List

from dateutil import parser as date_parser

from .session import BastionSession


OCI_COMMAND_TIMEOUT_SECONDS = 60


@dataclass
class TargetDetails:
    bastion_id: str
    instance_id: str
    private_ip: str
    target_user: str
    public_key_path: str


class BastionClient:
    def __init__(self, profile: str, region: str, auth_method: str) -> None:
        self.profile = profile
        self.region = region
        self.auth_method = auth_method

    def _run(self, *args: str) -> str:
        command: List[str] = ["oci", "--profile", self.profile]
        if self.region:
            command.extend(["--region", self.region])
        if self.auth_method:
            command.extend(["--auth", self.auth_method])
        command.extend(args)
        try:
            completed = subprocess.run(
                command,
                check=True,
                capture_output=True,
                text=True,
                timeout=OCI_COMMAND_TIMEOUT_SECONDS,
            )
        except subprocess.TimeoutExpired as exc:
            raise RuntimeError(
                "Timed out waiting for OCI CLI response; the security token may be missing or expired. "
                f"Run `oci session authenticate --profile {self.profile}` to refresh the token."
            ) from exc
        except subprocess.CalledProcessError as exc:
            stderr = exc.stderr or ""
            lowered = stderr.lower()
            if "security token" in lowered or "security_token" in lowered or "security-token" in lowered:
                raise RuntimeError(
                    "OCI CLI reported a security token authentication failure. "
                    f"Re-authenticate with `oci session authenticate --profile {self.profile}`."
                ) from exc
            raise
        return completed.stdout

    def create_session(self, target: TargetDetails) -> BastionSession:
        output = self._run(
            "bastion",
            "session",
            "create-managed-ssh",
            "--bastion-id",
            target.bastion_id,
            "--target-resource-id",
            target.instance_id,
            "--target-private-ip",
            target.private_ip,
            "--target-os-username",
            target.target_user,
            "--ssh-public-key-file",
            target.public_key_path,
            "--query",
            "data",
            "--raw-output",
        )
        return self._to_session(json.loads(output))

    def get_session(self, session_id: str) -> BastionSession:
        output = self._run(
            "bastion",
            "session",
            "get",
            "--session-id",
            session_id,
            "--query",
            "data",
            "--raw-output",
        )
        return self._to_session(json.loads(output))

    @staticmethod
    def _to_session(data: dict) -> BastionSession:
        def _get(*keys: str) -> str:
            for key in keys:
                if key in data:
                    return data[key]
            raise KeyError(keys[0])

        def _optional(*keys: str):
            for key in keys:
                if key in data:
                    return data[key]
            return None

        return BastionSession(
            id=_get("id"),
            lifecycle_state=_get("lifecycleState", "lifecycle-state", "lifecycle_state"),
            time_created=date_parser.isoparse(_get("timeCreated", "time-created", "time_created")),
            time_expires=_parse_expiry(
                _optional("timeExpires", "time-expires", "time_expires"),
                _optional("sessionTtlInSeconds", "session-ttl-in-seconds", "session_ttl_in_seconds"),
                _get("timeCreated", "time-created", "time_created"),
            ),
        )


def _parse_expiry(time_expires_raw, ttl_raw, time_created_raw):
    time_created = date_parser.isoparse(time_created_raw)
    if time_expires_raw:
        return date_parser.isoparse(time_expires_raw)

    if ttl_raw is not None:
        try:
            ttl_seconds = int(ttl_raw)
        except (TypeError, ValueError):
            ttl_seconds = 0
        if ttl_seconds > 0:
            return time_created + timedelta(seconds=ttl_seconds)

    return time_created + timedelta(hours=1)
