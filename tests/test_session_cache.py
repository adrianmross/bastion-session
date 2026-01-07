from datetime import datetime, timedelta, timezone

from bastion_session_cli.config import StateStore
from bastion_session_cli.session import BastionSession, SessionCache


def test_session_cache_roundtrip(tmp_path):
    state_file = tmp_path / "state.json"
    cache = SessionCache(StateStore(state_file))
    session = BastionSession(
        id="ocid1.bastionsession.oc1..example",
        lifecycle_state="ACTIVE",
        time_created=datetime.now(timezone.utc),
        time_expires=datetime.now(timezone.utc) + timedelta(hours=1),
    )

    cache.set(session)
    stored = cache.get()
    assert stored is not None
    assert stored.id == session.id
    assert stored.lifecycle_state == session.lifecycle_state
