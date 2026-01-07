from bastion_session_cli.config import Config


def test_config_from_env(monkeypatch) -> None:
    monkeypatch.setenv("PROFILE", "custom")
    monkeypatch.setenv("REGION", "eu-milan-1")
    cfg = Config.from_env()
    assert cfg.profile == "custom"
    assert cfg.region == "eu-milan-1"
