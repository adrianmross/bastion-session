from bastion_session_cli.ssh_config import render_fragment


def test_render_fragment() -> None:
    fragment = render_fragment([
        "Host example\n  HostName example.com\n  User test"
    ])
    assert fragment.startswith("# Managed by bastion-session CLI")
    assert "Host example" in fragment
