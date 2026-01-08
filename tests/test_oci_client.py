import json
from datetime import datetime

from bastion_session_cli.oci_client import BastionClient, TargetDetails


def test_to_session_handles_cli_keys(monkeypatch):
    client = BastionClient(profile="p", region="", auth_method="")
    cli_output = json.dumps({
        "id": "ocid1.session",
        "lifecycle-state": "ACTIVE",
        "time-created": "2024-06-01T12:00:00Z",
        "time-expires": "2024-06-01T13:00:00Z",
    })

    monkeypatch.setattr(
        client,
        "_run",
        lambda *args, **kwargs: cli_output,
    )

    session = client.create_session(
        TargetDetails(
            bastion_id="ocid1.bastion",
            instance_id="ocid1.instance",
            private_ip="10.0.0.5",
            target_user="opc",
            public_key_path="/tmp/key.pub",
        )
    )

    assert session.lifecycle_state == "ACTIVE"
    assert isinstance(session.time_created, datetime)
    assert isinstance(session.time_expires, datetime)


def test_to_session_handles_ttl_only(monkeypatch):
    client = BastionClient(profile="p", region="", auth_method="")
    cli_output = json.dumps({
        "id": "ocid1.session",
        "lifecycleState": "ACTIVE",
        "timeCreated": "2024-06-01T12:00:00Z",
        "sessionTtlInSeconds": 1800,
    })

    monkeypatch.setattr(client, "_run", lambda *args, **kwargs: cli_output)

    session = client.get_session("ocid1.session")

    assert session.time_expires.isoformat() == "2024-06-01T12:30:00+00:00"
