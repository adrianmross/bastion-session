"""Command-line interface entry for bastion session manager."""

from __future__ import annotations

from pathlib import Path

import click

from .config import Config
from .runtime import refresh_session, session_status, watch


@click.group()
@click.option("--profile", default=None, help="OCI profile name")
@click.option("--region", default=None, help="OCI region identifier")
@click.option("--auth-method", default=None, help="OCI auth method")
@click.option("--target-user", default=None, help="Target OS user")
@click.option("--ssh-public-key", default=None, type=click.Path(path_type=Path), help="Path to SSH public key")
@click.option("--ssh-include", default=None, type=click.Path(path_type=Path), help="Path to SSH include fragment")
@click.pass_context
def cli(ctx: click.Context, profile: str | None, region: str | None, auth_method: str | None,
        target_user: str | None, ssh_public_key: Path | None, ssh_include: Path | None) -> None:
    """Manage OCI bastion sessions and SSH config fragments."""
    config = Config.from_env()
    if profile:
        config.profile = profile
    if region:
        config.region = region
    if auth_method:
        config.auth_method = auth_method
    if target_user:
        config.target_user = target_user
    if ssh_public_key:
        config.ssh_public_key = ssh_public_key
    if ssh_include:
        config.ssh_include_path = ssh_include
    ctx.obj = config


@cli.command()
@click.pass_obj
def refresh(config: Config) -> None:
    """Create or refresh a bastion session."""
    refresh_session(config)


@cli.command()
@click.pass_obj
def status(config: Config) -> None:
    """Show cached session status."""
    session_status(config)


@cli.command()
@click.option("--interval", default=300, show_default=True, help="Refresh interval in seconds")
@click.pass_obj
def watch(config: Config, interval: int) -> None:
    """Continuously refresh session on an interval."""
    watch(config, interval_seconds=interval)
