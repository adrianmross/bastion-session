# Bastion Session CLI (Go)

Go-based CLI/TUI utility to manage OCI bastion managed SSH sessions, maintain SSH config fragments,
and keep sessions refreshed for remote workstation access.

## Features

- Create OCI bastion managed SSH sessions (via OCI CLI).
- Cache session metadata and render SSH config include files.
- Watch mode with auto-refresh based on TTL.
- OCI-context-aware scoping (profile/region/compartment from current `oci-context`).
- List bastions in current scoped context, plus tracked bastions history.
- Interactive TUI (based on `oci-context` patterns) with scope banner and escape-to-tracked mode.

## Build

```bash
cd bastion-session
go build -o bastion-session ./cmd/bastion-session
```

## Usage

```bash
./bastion-session refresh --ssh-public-key ~/.ssh/keys/mykey.pub
./bastion-session status
./bastion-session watch --interval 600
./bastion-session list-bastions --source scoped
./bastion-session tui
```

## Python Deprecation

- The legacy Python implementation has been removed from this repository.
- `bastion-session` is now Go-only (CLI + TUI).

Ensure your `~/.ssh/config` includes:

```
Include ~/.ssh/config.d/bastion-session
```

## Context Scoping

By default, bastion-session loads your current `oci-context` and scopes operations by it
(profile, auth method, region, compartment).

- Disable scoping for a command: `--no-context-scope`
- Force global oci-context config: `--global`
- Use explicit oci-context file: `--oci-context-config /path/to/config.yml`

In TUI:

- Scoped mode is default (banner shows active context).
- Press `e` to escape to tracked bastions.
- Press `s` to return to scoped bastions.
- Press `r` to refresh list.
- Press `Enter` to select a bastion ID.

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
