import json
import subprocess
from datetime import datetime

import pytest

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



def test_run_timeout_raises_helpful_error(monkeypatch):
    client = BastionClient(profile="p", region="", auth_method="")

    def fake_run(*args, **kwargs):
        raise subprocess.TimeoutExpired(cmd=args[0], timeout=60)

    monkeypatch.setattr("bastion_session_cli.oci_client.subprocess.run", fake_run)

    with pytest.raises(RuntimeError) as exc:
        client._run("bastion", "session")

    message = str(exc.value)
    assert "timed out waiting" in message.lower()
    assert "oci session authenticate" in message


def test_run_token_error_prompts_reauth(monkeypatch):
    client = BastionClient(profile="p", region="", auth_method="")

    def fake_run(*args, **kwargs):
        raise subprocess.CalledProcessError(
            1,
            args[0],
            stderr="ERROR: The security token has expired."
        )

    monkeypatch.setattr("bastion_session_cli.oci_client.subprocess.run", fake_run)

    with pytest.raises(RuntimeError) as exc:
        client._run("bastion", "session")

    message = str(exc.value)
    assert "security token" in message.lower()
    assert "oci session authenticate" in message
