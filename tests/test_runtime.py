import json
from pathlib import Path

from bastion_session_cli.config import Config
from bastion_session_cli.runtime import load_target_details
from bastion_session_cli.terraform import read_outputs


def make_state_file(path: Path) -> Path:
    data = {
        "outputs": {
            "bastion_id": {"value": "ocid1.bastion"},
            "instance_id": {"value": "ocid1.instance"},
            "private_ip": {"value": "10.0.0.10"},
            "bastion_public_ip": {"value": "198.51.100.10"},
        }
    }
    path.write_text(json.dumps(data))
    return path


def test_read_outputs_accepts_tfstate(tmp_path):
    state_path = make_state_file(tmp_path / "terraform.tfstate")
    outputs = read_outputs(state_path)
    assert outputs["bastion_id"] == "ocid1.bastion"
    assert outputs["private_ip"] == "10.0.0.10"


def test_load_target_details_with_env(monkeypatch, tmp_path):
    state_path = make_state_file(tmp_path / "terraform.tfstate")
    monkeypatch.setenv("TERRAFORM_OUTPUTS", str(state_path))

    config = Config.from_env()
    metadata = load_target_details(config)

    assert metadata.bastion_id == "ocid1.bastion"
    assert metadata.instance_id == "ocid1.instance"
    assert metadata.private_ip == "10.0.0.10"
    assert metadata.bastion_host == "198.51.100.10"
