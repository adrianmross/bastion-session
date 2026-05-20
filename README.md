# Bastion Session CLI

Manage OCI Bastion managed SSH sessions and keep durable `ssh <host>` aliases
working for private OCI compute instances.

![bastion-session terminal demo](docs/assets/bastion-session-demo.gif)

`bastion-session` is the lower-level Go CLI that creates sessions, tracks
compute targets, renders SSH config fragments, and explains what will happen
before an operator connects.

## What It Does

- creates OCI Bastion managed SSH sessions through the OCI CLI
- caches session metadata
- writes SSH config include files
- tracks compute targets by hostname
- renews and prunes expiring sessions
- audits effective SSH config
- scopes operations from the active `oci-context`
- returns stable JSON for scripts and agents

## Install

Homebrew is the preferred install path:

```bash
brew tap adrianmross/tap
brew install bastion-session
```

The Homebrew binary is installed at:

```bash
/opt/homebrew/bin/bastion-session
```

Source install:

```bash
curl -sSL https://raw.githubusercontent.com/adrianmross/bastion-session/main/install.sh | bash
```

By default the installer writes to `/usr/local/bin`. Override it with `PREFIX`:

```bash
PREFIX="$HOME/.local" curl -sSL https://raw.githubusercontent.com/adrianmross/bastion-session/main/install.sh | bash
```

Install a specific release:

```bash
VERSION=v0.8.0 curl -sSL https://raw.githubusercontent.com/adrianmross/bastion-session/main/install.sh | bash
```

## Quickstart

Make sure your SSH config includes the managed fragment:

```sshconfig
Include ~/.ssh/config.d/bastion-session
```

Track a compute target:

```bash
bastion-session target track my-vps-01 \
  --instance-id ocid1.instance.oc1..example \
  --private-ip 10.0.1.25 \
  --user cloud-user \
  --identity-file ~/.ssh/oci/example-vm.key \
  --bastion-id ocid1.bastion.oc1..example
```

Or import it from Terraform outputs containing `bastion_id`, `instance_id`, and
`private_ip`:

```bash
bastion-session target import my-vps-01 --terraform-outputs ./terraform-directory
```

Create or reuse the session and write the VM-facing SSH host:

```bash
bastion-session ensure my-vps-01
```

Request a longer TTL for sessions that need to be created:

```bash
bastion-session ensure my-vps-01 --session-ttl 3h
bastion-session session new my-bastion --session-ttl 10800
```

`--session-ttl` accepts Go-style durations or seconds and is passed to OCI as
`--session-ttl-in-seconds`. Existing healthy sessions are still reused; the TTL
only applies when a new managed SSH session is created.

Connect to the compute host:

```bash
ssh my-vps-01
```

## Host-Facing SSH

The generated include file keeps the internal bastion jump host current while
preserving durable VM aliases:

```sshconfig
Host my-vps-01
  HostName 10.0.1.25
  User cloud-user
  ProxyJump my-bastion
```

Operators connect to `my-vps-01`; session rotation only changes the managed
bastion internals.

## Common Commands

```bash
bastion-session --version
bastion-session --version --json
bastion-session version -o json
bastion-session paths -o json
bastion-session list --source scoped
bastion-session list --source tracked
bastion-session use <ref-or-bastion-ocid> --source tracked --key ~/.ssh/id_ed25519.pub
bastion-session current
bastion-session connect -o json
bastion-session ensure my-vps-01 -o json
bastion-session target import my-vps-01 --terraform-outputs ./terraform-directory
bastion-session target reconcile my-vps-01 --cached -o json
bastion-session target list -o table
bastion-session target show my-vps-01 -o json
bastion-session ssh-config show my-vps-01 -o json
bastion-session ssh-config audit my-vps-01 -o json
bastion-session doctor my-vps-01 -o json
bastion-session explain my-vps-01 -o json
bastion-session session list
bastion-session session new <bastion-ref> -o json
bastion-session session renew my-vps-01 -o json
bastion-session session prune -o json
bastion-session tui
```

## Paths

Use `paths` for script-safe local metadata:

```bash
bastion-session paths -o json
```

Typical paths:

- Homebrew binary: `/opt/homebrew/bin/bastion-session`
- Source install binary: `/usr/local/bin/bastion-session`
- SSH include: `~/.ssh/config.d/bastion-session`
- session cache: `~/.cache/bastion-session/state.json`
- tracked targets: `~/.cache/bastion-session/tracked-targets.json`
- tracked bastions: `~/.cache/bastion-session/tracked-bastions.json`
- current bastion: `~/.cache/bastion-session/current-bastion.json`

Override tracked targets with `BASTION_TRACKED_TARGETS_PATH` or
`--tracked-targets-path`.

## Recovery And Diagnostics

Use `explain` when you want operator-oriented state without treating warnings
as command failure:

```bash
bastion-session explain my-vps-01 -o json
```

Use `doctor` for health checks:

```bash
bastion-session doctor my-vps-01 -o json
bastion-session doctor my-vps-01 --cached -o json
```

Use `target reconcile` when a host already works through an active bastion
session but is missing from the tracked target registry:

```bash
bastion-session target reconcile my-vps-01 --cached -o json
```

Use `ssh-config audit` before editing SSH config:

```bash
bastion-session ssh-config audit my-vps-01 -o json
```

`status` and `doctor` warn when a cached or live session is within the
near-expiry window. `session prune` removes expired cached session state.
`session renew <host>` follows the same create/reuse path as `ensure <host>`.

## Context Scoping

By default, `bastion-session` loads the current `oci-context` and scopes OCI
operations by profile, auth method, region, and compartment.

```bash
bastion-session list --source scoped
bastion-session --no-context-scope list
bastion-session --global list
bastion-session --oci-context-config /path/to/config.yml list
```

In TUI:

- scoped mode is default
- `e` escapes to tracked bastions
- `s` returns to scoped bastions
- `r` refreshes
- `Enter` selects a bastion ID

## Agent Contract

Stable automation output is JSON. Agents should prefer `-o json` for supported
commands such as `connect`, `ensure`, `explain`, `paths`, `session new`, and
`target` commands. Human-readable output is for operators, not parsers.

JSON keys are compatibility surface. Add fields when needed, but avoid renaming
or removing existing fields without a documented migration path.

## Background Services

Service files are generated by CLI commands and are not vendored in this
repository.

Generate and install a macOS launchd service:

```bash
bastion-session service launchd install --interval 300
```

Generate and install a systemd user service:

```bash
bastion-session service systemd install --interval 300
```

Default launchd logs:

```text
~/.bastion-session/watch.out.log
~/.bastion-session/watch.err.log
```

## Development

```bash
go build -o bastion-session ./cmd/bastion-session
make fmt
make vet
make test
make lint-workflows
make validate-workflows
```

## Release Automation

- CI: `.github/workflows/ci.yml`
- alpha preview artifacts on PRs: `.github/workflows/prerelease-alpha.yml`
- tagged releases via GoReleaser: `.github/workflows/release.yml`
- automatic semantic tagging on `main`: `.github/workflows/auto-release.yml`

The legacy Python implementation has been removed. `bastion-session` is Go-only.
