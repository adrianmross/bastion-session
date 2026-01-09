from __future__ import annotations

from datetime import timedelta

import pytest

from bastion_session_cli import runtime
from bastion_session_cli.config import Config


class DummySession:
    def __init__(self, expires_in_seconds: float) -> None:
        self._expires_in = timedelta(seconds=expires_in_seconds)

    @property
    def expires_in(self):
        return self._expires_in


def test_auto_refresh_interval_respects_margin():
    margin = runtime.AUTO_REFRESH_MARGIN.total_seconds()
    session = DummySession(expires_in_seconds=margin + 600)
    expected = int(session.expires_in.total_seconds() - margin)
    assert runtime._auto_refresh_interval(session) == expected


def test_auto_refresh_interval_clamps_minimum():
    margin = runtime.AUTO_REFRESH_MARGIN.total_seconds()
    session = DummySession(expires_in_seconds=margin - 5)
    assert runtime._auto_refresh_interval(session) == runtime.MIN_AUTO_REFRESH_SECONDS


def test_watch_uses_auto_interval_when_unspecified(monkeypatch):
    config = Config()
    expected_sleep = 123
    sleep_calls = {}

    monkeypatch.setattr(runtime, "refresh_session", lambda cfg: DummySession(expected_sleep + runtime.AUTO_REFRESH_MARGIN.total_seconds()))
    monkeypatch.setattr(runtime, "_auto_refresh_interval", lambda session: expected_sleep)
    monkeypatch.setattr(runtime.console, "print", lambda *args, **kwargs: None)

    def fake_sleep(seconds):
        sleep_calls["value"] = seconds
        raise RuntimeError("stop")

    monkeypatch.setattr(runtime.time, "sleep", fake_sleep)

    with pytest.raises(RuntimeError, match="stop"):
        runtime.watch(config, interval_seconds=None)

    assert sleep_calls["value"] == expected_sleep


def test_watch_honors_explicit_interval(monkeypatch):
    config = Config()
    explicit_interval = 45
    sleep_calls = {}

    monkeypatch.setattr(runtime, "refresh_session", lambda cfg: DummySession(explicit_interval))

    def fail_auto(session):
        raise AssertionError("auto interval should not be used")

    monkeypatch.setattr(runtime, "_auto_refresh_interval", fail_auto)
    monkeypatch.setattr(runtime.console, "print", lambda *args, **kwargs: None)

    def fake_sleep(seconds):
        sleep_calls["value"] = seconds
        raise RuntimeError("stop")

    monkeypatch.setattr(runtime.time, "sleep", fake_sleep)

    with pytest.raises(RuntimeError, match="stop"):
        runtime.watch(config, interval_seconds=explicit_interval)

    assert sleep_calls["value"] == explicit_interval
