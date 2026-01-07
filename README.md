# Bastion Session CLI

CLI utility to manage OCI bastion managed SSH sessions, maintain SSH config fragments,
and keep sessions refreshed for remote workstation access.

## Features

- Create bastion managed SSH sessions using OCI SDK.
- Cache session metadata and render SSH config include files.
- Watch mode to auto-refresh sessions on a schedule.
- Environment overrides and CLI options for profile, region, and user.

## Installation

```bash
./scripts/install-bastion-cli.sh --editable
```

## Usage

```bash
bastion-session refresh --ssh-public-key ~/.ssh/keys/mykey.pub
bastion-session status
bastion-session watch --interval 600
```

Ensure your `~/.ssh/config` includes:

```
Include ~/.ssh/config.d/bastion-session
```

Optionally supply a path to Terraform outputs via `TERRAFORM_OUTPUTS` environment variable.

## Background Services

### systemd --user

1. Copy `assets/systemd/bastion-session.service` to `~/.config/systemd/user/`.
2. Adjust environment variables or CLI arguments as needed.
3. Reload and enable:
   ```bash
   systemctl --user daemon-reload
   systemctl --user enable --now bastion-session.service
   ```
4. Check status with `systemctl --user status bastion-session`.

### launchd (macOS)

1. Copy `assets/launchd/com.remote.bastion-session.plist` to `~/Library/LaunchAgents/`.
2. Edit key paths if your `bastion-session` binary lives elsewhere.
3. Load the agent:
   ```bash
   launchctl load ~/Library/LaunchAgents/com.remote.bastion-session.plist
   ```
4. View logs at `/tmp/bastion-session.log` and `/tmp/bastion-session.err.log`.
