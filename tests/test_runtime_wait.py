import json
from datetime import datetime, timedelta, timezone

from bastion_session_cli import runtime
from bastion_session_cli.config import Config
from bastion_session_cli.session import BastionSession


def test_refresh_session_waits_for_active(monkeypatch, tmp_path):
    config = Config()
    config.session_state_path = tmp_path / "state.json"
    config.ssh_include_path = tmp_path / "ssh" / "fragment"
    config.ssh_public_key = tmp_path / "id_rsa.pub"
    config.ssh_public_key.write_text("ssh-rsa AAA")

    metadata = runtime.SessionMetadata(
        bastion_id="ocid1.bastion",
        instance_id="ocid1.instance",
        private_ip="10.0.0.10",
        bastion_host="198.51.100.10",
    )
    monkeypatch.setattr(runtime, "load_target_details", lambda cfg: metadata)
    monkeypatch.setattr(runtime, "ensure_includes", lambda cfg: None)

    update_calls = {}

    def fake_update(cfg, meta, session_id):
        update_calls["session_id"] = session_id
        update_calls["metadata"] = meta

    monkeypatch.setattr(runtime, "update_ssh_fragment", fake_update)

    now = datetime.now(timezone.utc)
    expires = now + timedelta(minutes=30)
    sessions = [
        BastionSession("session-123", "CREATING", now, expires),
        BastionSession("session-123", "CREATING", now, expires),
        BastionSession("session-123", "ACTIVE", now, expires),
    ]

    class DummyClient:
        def __init__(self):
            self.create_calls = 0
            self.get_calls = 0

        def create_session(self, target):
            self.create_calls += 1
            return sessions[0]

        def get_session(self, session_id):
            self.get_calls += 1
            index = min(self.get_calls, len(sessions) - 1)
            return sessions[index]

    client = DummyClient()
    monkeypatch.setattr(runtime, "BastionClient", lambda *args, **kwargs: client)
    monkeypatch.setattr(runtime.time, "sleep", lambda _: None)

    runtime.refresh_session(config)

    state_data = json.loads(config.session_state_path.read_text())
    assert state_data["session"]["lifecycle_state"] == "ACTIVE"
    assert update_calls["session_id"] == "session-123"
    assert client.create_calls == 1
    assert client.get_calls >= 2
