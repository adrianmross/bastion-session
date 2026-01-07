"""Utilities for managing SSH config fragments."""

from __future__ import annotations

from pathlib import Path
from typing import Iterable

SSH_HEADER = "# Managed by bastion-session CLI\n"


def render_fragment(host_entries: Iterable[str]) -> str:
    body = "\n".join(host_entries)
    return f"{SSH_HEADER}{body}\n"


def write_fragment(path: Path, host_entries: Iterable[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    content = render_fragment(host_entries)
    tmp = path.with_suffix(".tmp")
    tmp.write_text(content)
    tmp.replace(path)
