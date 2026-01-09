"""Tests for CLI version option."""

from __future__ import annotations

from pathlib import Path

import sys

import pytest
from click.testing import CliRunner


@pytest.fixture(autouse=True)
def _add_src(monkeypatch: pytest.MonkeyPatch) -> None:
    src_path = Path(__file__).resolve().parents[1] / "src"
    monkeypatch.syspath_prepend(str(src_path))


def test_version_option() -> None:
    from bastion_session_cli import __version__
    from bastion_session_cli.main import cli

    result = CliRunner().invoke(cli, ["--version"])
    assert result.exit_code == 0
    assert __version__ in result.output

