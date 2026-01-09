"""Tests for the watch CLI command."""

from __future__ import annotations

from pathlib import Path

import pytest
from click.testing import CliRunner


@pytest.fixture(autouse=True)
def _add_src(monkeypatch: pytest.MonkeyPatch) -> None:
    src_path = Path(__file__).resolve().parents[1] / "src"
    monkeypatch.syspath_prepend(str(src_path))


def test_watch_invokes_runtime(monkeypatch: pytest.MonkeyPatch) -> None:
    from bastion_session_cli.main import cli

    called = {}

    def fake_watch(config, interval_seconds: int) -> None:
        called["config"] = config
        called["interval"] = interval_seconds

    monkeypatch.setattr("bastion_session_cli.main.runtime_watch", fake_watch)

    result = CliRunner().invoke(cli, ["watch", "-i", "123"])
    assert result.exit_code == 0
    assert called["interval"] == 123
    assert called["config"].target_user == "opc"
