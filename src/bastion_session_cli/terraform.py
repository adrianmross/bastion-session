"""Helpers for retrieving Terraform outputs."""

from __future__ import annotations

from pathlib import Path
from typing import Dict
import json
import subprocess


def read_outputs(path: Path) -> Dict[str, str]:
    data = json.loads(path.read_text())
    return {key: value["value"] for key, value in data.items()}


def terraform_output_raw(name: str) -> str:
    result = subprocess.run([
        "terraform",
        "output",
        "-raw",
        name,
    ], check=True, capture_output=True, text=True)
    return result.stdout.strip()
