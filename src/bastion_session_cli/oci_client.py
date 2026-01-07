"""Wrapper around OCI SDK interactions."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime

import oci

from .session import BastionSession


def build_config(profile: str, region: str, auth: str) -> dict:
    config = oci.config.from_file(profile_name=profile)
    config["region"] = region
    config["auth_type"] = auth
    return config


@dataclass
class TargetDetails:
    bastion_id: str
    instance_id: str
    private_ip: str
    target_user: str
    public_key_path: str


class BastionClient:
    def __init__(self, config: dict) -> None:
        self.client = oci.bastion.BastionClient(config)

    def create_session(self, target: TargetDetails) -> BastionSession:
        response = self.client.create_session(
            oci.bastion.models.CreateBastionSessionDetails(
                bastion_id=target.bastion_id,
                key_details=oci.bastion.models.PublicKeyDetails(public_key_content=open(target.public_key_path).read()),
                target_resource_details=oci.bastion.models.CreateSessionTargetResourceDetails(
                    session_type="MANAGED_SSH",
                    target_resource_id=target.instance_id,
                    target_resource_operating_system_user_name=target.target_user,
                    target_resource_private_ip_address=target.private_ip,
                ),
                display_name=f"bastion-session-cli-{datetime.utcnow().isoformat()}",
                key_type="PUB",
            )
        )
        return self._to_session(response.data)

    def get_session(self, session_id: str) -> BastionSession:
        response = self.client.get_session(session_id)
        return self._to_session(response.data)

    @staticmethod
    def _to_session(data) -> BastionSession:
        return BastionSession(
            id=data.id,
            lifecycle_state=data.lifecycle_state,
            time_created=data.time_created,
            time_expires=data.time_expires,
        )
