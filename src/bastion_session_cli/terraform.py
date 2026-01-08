"""Helpers for retrieving Terraform outputs."""

from __future__ import annotations

from pathlib import Path
from typing import Dict, Any
import json
import subprocess


def read_outputs(path: Path) -> Dict[str, Any]:
    data = json.loads(path.read_text())
    if isinstance(data, dict) and "outputs" in data:
        data = data["outputs"]

    result: Dict[str, Any] = {}
    for key, value in data.items():
        if isinstance(value, dict) and "value" in value:
            result[key] = value["value"]
        else:
            result[key] = value
    return result


def terraform_output_raw(name: str) -> str:
    result = subprocess.run([
        "terraform",
        "output",
        "-raw",
        name,
    ], check=True, capture_output=True, text=True)
    return result.stdout.strip()
