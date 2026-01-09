"""Command-line interface entry for bastion session manager."""

from __future__ import annotations

from pathlib import Path

import click

from . import __version__
from .config import Config
from .runtime import refresh_session, session_status, watch as runtime_watch


@click.group()
@click.version_option(__version__, "-v", "--version", prog_name="bastion-session")
@click.option("-p", "--profile", default=None, help="OCI profile name")
@click.option("-r", "--region", default=None, help="OCI region identifier")
@click.option("-a", "--auth-method", default=None, help="OCI auth method")
@click.option("-u", "--target-user", default=None, help="Target OS user")
@click.option("-P", "--ssh-public-key", default=None, type=click.Path(path_type=Path), help="Path to SSH public key")
@click.option("-K", "--ssh-private-key", default=None, type=click.Path(path_type=Path), help="Path to SSH private key")
@click.option("-I", "--ssh-include", default=None, type=click.Path(path_type=Path), help="Path to SSH include fragment")
@click.pass_context
def cli(ctx: click.Context, profile: str | None, region: str | None, auth_method: str | None,
        target_user: str | None, ssh_public_key: Path | None, ssh_private_key: Path | None, ssh_include: Path | None) -> None:
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
    if ssh_private_key:
        config.ssh_private_key = ssh_private_key
    if ssh_include:
        config.ssh_include_path = ssh_include
    ctx.obj = config


@cli.command()
@click.pass_obj
def refresh(config: Config) -> None:
    """Create or refresh a bastion session."""
    refresh_session(config)


@cli.command()
@click.option(
    "-o",
    "--output",
    "output_format",
    type=click.Choice(["table", "json", "yaml"], case_sensitive=False),
    default="table",
    show_default=True,
    help="Output format",
)
@click.pass_obj
def status(config: Config, output_format: str) -> None:
    """Show session status."""
    session_status(config, output_format)


@cli.command()
@click.option("-i", "--interval", default=300, show_default=True, help="Refresh interval in seconds")
@click.pass_obj
def watch(config: Config, interval: int) -> None:
    """Continuously refresh session on an interval."""
    runtime_watch(config, interval_seconds=interval)
