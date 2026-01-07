"""Configuration handling for bastion session CLI."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Optional
import json
import os

DEFAULT_CONFIG_PATH = Path.home() / ".config" / "bastion-session" / "config.json"
DEFAULT_STATE_PATH = Path.home() / ".cache" / "bastion-session" / "state.json"
DEFAULT_SSH_INCLUDE = Path.home() / ".ssh" / "config.d" / "bastion-session"


@dataclass
class Config:
    profile: str = "oabcs1-terraform"
    region: str = "us-chicago-1"
    auth_method: str = "security_token"
    target_user: str = "opc"
    ssh_public_key: Optional[Path] = None
    ssh_include_path: Path = DEFAULT_SSH_INCLUDE
    session_state_path: Path = DEFAULT_STATE_PATH
    terraform_outputs_path: Optional[Path] = None

    @staticmethod
    def from_env() -> "Config":
        return Config(
            profile=os.getenv("PROFILE", Config.profile),
            region=os.getenv("REGION", Config.region),
            auth_method=os.getenv("AUTH_METHOD", Config.auth_method),
            target_user=os.getenv("TARGET_USER", Config.target_user),
            ssh_public_key=Path(os.getenv("SSH_PUBLIC_KEY")) if os.getenv("SSH_PUBLIC_KEY") else None,
            terraform_outputs_path=Path(os.getenv("TERRAFORM_OUTPUTS")) if os.getenv("TERRAFORM_OUTPUTS") else None,
        )


class StateStore:
    """Persist session state to disk."""

    def __init__(self, path: Path = DEFAULT_STATE_PATH) -> None:
        self.path = path

    def load(self) -> dict:
        if not self.path.exists():
            return {}
        try:
            return json.loads(self.path.read_text())
        except json.JSONDecodeError:
            return {}

    def save(self, data: dict) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = self.path.with_suffix(".tmp")
        temp_path.write_text(json.dumps(data, indent=2))
        temp_path.replace(self.path)
