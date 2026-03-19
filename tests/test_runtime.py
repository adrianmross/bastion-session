import json
from pathlib import Path

from bastion_session_cli.config import Config
from bastion_session_cli import runtime
from bastion_session_cli.runtime import load_target_details
from bastion_session_cli.terraform import read_outputs


def make_state_file(path: Path, public_key_path: str | None = None) -> Path:
    data = {
        "outputs": {
            "bastion_id": {"value": "ocid1.bastion"},
            "instance_id": {"value": "ocid1.instance"},
            "private_ip": {"value": "10.0.0.10"},
            "bastion_public_ip": {"value": "198.51.100.10"},
        }
    }
    if public_key_path:
        data["outputs"]["bastion_ssh_public_key_path"] = {"value": public_key_path}
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


def test_load_target_details_parent_search(monkeypatch, tmp_path):
    tf_root = tmp_path / "tf" / "dev-remote-workstation"
    tf_root.mkdir(parents=True)
    state_path = make_state_file(tf_root / "terraform.tfstate")

    nested_dir = tf_root / "subdir"
    nested_dir.mkdir()
    monkeypatch.chdir(nested_dir)

    config = Config()
    metadata = load_target_details(config)

    assert metadata.bastion_id == "ocid1.bastion"
    assert metadata.instance_id == "ocid1.instance"
    assert metadata.private_ip == "10.0.0.10"
    assert metadata.bastion_host == "198.51.100.10"


def test_resolve_public_key_from_env(monkeypatch, tmp_path):
    pub_key = tmp_path / "id_rsa.pub"
    pub_key.write_text("ssh-rsa AAA")

    config = Config()
    monkeypatch.delenv("SSH_PUBLIC_KEY", raising=False)
    monkeypatch.setenv("SSH_PUBLIC_KEY", str(pub_key))

    resolved = runtime._resolve_public_key(config)

    assert resolved == pub_key


def test_resolve_public_key_from_outputs(tmp_path):
    tf_root = tmp_path / "tf" / "dev-remote-workstation"
    tf_root.mkdir(parents=True)
    pub_key = tf_root / "bastion.pub"
    pub_key.write_text("ssh-rsa AAA")
    state_path = make_state_file(tf_root / "terraform.tfstate", public_key_path=str(pub_key))

    config = Config()
    config.terraform_outputs_path = state_path

    resolved = runtime._resolve_public_key(config)

    assert resolved == pub_key


def test_resolve_public_key_from_tfvars(monkeypatch, tmp_path):
    tf_root = tmp_path / "tf" / "dev-remote-workstation"
    tf_root.mkdir(parents=True)
    pub_dir = tf_root / "ssh"
    pub_dir.mkdir()
    pub_key = pub_dir / "bastion.pub"
    pub_key.write_text("ssh-rsa AAA")

    state_path = make_state_file(tf_root / "terraform.tfstate")
    (tf_root / "env.tfvars").write_text(
        'bastion_ssh_public_key_path = "ssh/bastion.pub"\n'
    )

    config = Config()
    config.terraform_outputs_path = state_path

    monkeypatch.chdir(tf_root)

    resolved = runtime._resolve_public_key(config)

    assert resolved == pub_key
