"""OCI bastion session management."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Optional

from dateutil import parser as date_parser


@dataclass
class BastionSession:
    id: str
    lifecycle_state: str
    time_created: datetime
    time_expires: datetime

    @property
    def expires_in(self) -> timedelta:
        return self.time_expires - datetime.now(timezone.utc)


class SessionCache:
    def __init__(self, state_store) -> None:
        self.state_store = state_store

    def get(self) -> Optional[BastionSession]:
        data = self.state_store.load()
        session_data = data.get("session")
        if not session_data:
            return None
        try:
            return BastionSession(
                id=session_data["id"],
                lifecycle_state=session_data["lifecycle_state"],
                time_created=date_parser.isoparse(session_data["time_created"]),
                time_expires=date_parser.isoparse(session_data["time_expires"]),
            )
        except Exception:
            return None

    def set(self, session: BastionSession) -> None:
        self.state_store.save({
            "session": {
                "id": session.id,
                "lifecycle_state": session.lifecycle_state,
                "time_created": session.time_created.isoformat(),
                "time_expires": session.time_expires.isoformat(),
            }
        })
