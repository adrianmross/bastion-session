# Bastion Session CLI (Go)

Go-based CLI/TUI utility to manage OCI bastion managed SSH sessions, maintain SSH config fragments,
and keep sessions refreshed for remote workstation access.

## Features

- Create OCI bastion managed SSH sessions (via OCI CLI).
- Cache session metadata and render SSH config include files.
- Watch mode with auto-refresh based on TTL.
- OCI-context-aware scoping (profile/region/compartment from current `oci-context`).
- List bastions in current scoped context, plus tracked bastions history.
- Track VM targets by SSH alias for repeatable `ensure <host>` runs.
- Interactive TUI (based on `oci-context` patterns) with scope banner and escape-to-tracked mode.

## Build

```bash
cd bastion-session
go build -o bastion-session ./cmd/bastion-session
```

## Install

One-line install (latest stable):

```bash
curl -sSL https://raw.githubusercontent.com/adrianmross/bastion-session/main/install.sh | bash
```

Install a specific version:

```bash
VERSION=v0.1.0 curl -sSL https://raw.githubusercontent.com/adrianmross/bastion-session/main/install.sh | bash
```

## Usage

```bash
./bastion-session --version
./bastion-session refresh --ssh-public-key ~/.ssh/keys/mykey.pub
./bastion-session status
./bastion-session watch --interval 600
./bastion-session list --source scoped
./bastion-session list --source tracked
# Use either full OCID or short unique ref from list (2-3 chars when possible)
./bastion-session use <ref-or-bastion-ocid> --source tracked --key ~/.ssh/id_ed25519.pub
./bastion-session current
./bastion-session connect                       # create/refresh and connect
./bastion-session connect --key ~/.ssh/id_ed25519.pub
./bastion-session connect --session <sess-ref>  # reuse existing session
./bastion-session connect -o json
./bastion-session ensure vmordws02              # create/refresh and write VM-facing SSH host
./bastion-session ensure vmordws02 -o json
./bastion-session target track vmordws02 --instance-id ocid1.instance... --private-ip 10.42.1.217 --bastion-id ocid1.bastion...
./bastion-session target import vmordws02 --terraform-outputs ./terraform.tfstate
./bastion-session target list -o table
./bastion-session target show vmordws02 -o json
./bastion-session target rm vmordws02
./bastion-session ssh-config show vmordws02 -o json
./bastion-session doctor vmordws02 -o json
./bastion-session session list
./bastion-session session new <bastion-ref>
./bastion-session session new <bastion-ref> -o json
./bastion-session session new <bastion-ref> --key ~/.ssh/id_ed25519.pub
./bastion-session session use <session-id-or-ref>
./bastion-session track rm <ref-or-ocid>
./bastion-session track prune
./bastion-session tui
./bastion-session service systemd generate --interval 300
./bastion-session service launchd generate --interval 300
```

## Python Deprecation

- The legacy Python implementation has been removed from this repository.
- `bastion-session` is now Go-only (CLI + TUI).

Ensure your `~/.ssh/config` includes:

```
Include ~/.ssh/config.d/bastion-session
```

### VM-facing SSH aliases

`ensure` creates or reuses a managed SSH session, updates the internal OCI
bastion host alias, and writes a target VM alias that connects through it:

```bash
bastion-session ensure vmordws02 \
  --target-identity-file ~/.ssh/oci/example-vm.key
ssh vmordws02
```

Structured output is available for scripts and agents:

```bash
bastion-session ensure vmordws02 -o json
```

The generated SSH fragment keeps the internal `PROFILE-bastion` alias current
when sessions rotate, while preserving VM-facing aliases such as `vmordws02`.

### Tracked VM targets

Tracked targets persist under `~/.cache/bastion-session/tracked-targets.json`
by default. Override with `BASTION_TRACKED_TARGETS_PATH` or
`--tracked-targets-path`.

```bash
bastion-session target track vmordws02 \
  --instance-id ocid1.instance.oc1..example \
  --private-ip 10.42.1.217 \
  --user opc \
  --identity-file ~/.ssh/oci/example-vm.key \
  --bastion-id ocid1.bastion.oc1..example
```

Targets can also be populated from Terraform outputs containing `bastion_id`,
`instance_id`, and `private_ip`:

```bash
bastion-session target import vmordws02 --terraform-outputs ./outputs.json
bastion-session target import vmordws02 --terraform-outputs ./terraform-directory
```

After tracking, `bastion-session ensure vmordws02` fills the target instance,
private IP, target user, target identity file, and bastion ID from the registry
unless those values are supplied explicitly on the command line.

### Diagnostics

Use `doctor` for a local health summary. By default it includes a live OCI
session lookup when cached session state exists. Use `--cached` or `--no-live`
when an agent only needs local state and should avoid OCI API calls.

```bash
bastion-session doctor vmordws02 -o json
bastion-session doctor vmordws02 --cached -o json
bastion-session ssh-config show vmordws02 -o json
```

Doctor reports machine-readable `issues` and exits nonzero when it finds broken
state. Exit code `2` indicates selection/target state, `3` indicates session
state, and `4` indicates SSH configuration state.

For safe local repairs, `doctor --fix` can recreate the SSH include file and,
when a host plus cached active session are available, regenerate the SSH fragment.
It does not create OCI sessions; use `ensure <host>` for that.

```bash
bastion-session doctor vmordws02 --fix -o json
```

## Agent Contract

- Stable automation output is JSON. Agents should prefer `-o json` for supported
  commands such as `connect`, `ensure`, and `session new`, and should not parse
  human-readable output when JSON is available.
- JSON keys are compatibility surface. Add fields when needed, but avoid
  renaming or removing existing fields without a documented migration path.
- Preferred local checks are `make fmt`, `make vet`, `make test`,
  `make lint-workflows`, and `make validate-workflows`.
- Releases are produced from semantic `v*` tags through GoReleaser. The
  `auto-release` workflow may create the next tag from Conventional Commit
  subjects on `main`, but it skips commits that modify workflow files.

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

Service files are generated by CLI commands and are not vendored in this repository.

### systemd --user

Generate and install with automation:

```bash
./bastion-session service systemd install --interval 300
```

Generate only:

```bash
./bastion-session service systemd generate --interval 300
```

Manual enable/start:

```bash
systemctl --user daemon-reload
systemctl --user enable --now bastion-session
systemctl --user status bastion-session
```

### launchd (macOS)

Generate and install with automation:

```bash
./bastion-session service launchd install --interval 300
```

Generate only:

```bash
./bastion-session service launchd generate --interval 300
```

Manual load/start:

```bash
launchctl unload ~/Library/LaunchAgents/com.remote.bastion-session.plist 2>/dev/null || true
launchctl load ~/Library/LaunchAgents/com.remote.bastion-session.plist
launchctl start com.remote.bastion-session
```

Default logs from generated launchd plist:

```bash
~/.bastion-session/watch.out.log
~/.bastion-session/watch.err.log
```

## Release Automation

- CI: `.github/workflows/ci.yml`
- Alpha preview artifacts on PRs: `.github/workflows/prerelease-alpha.yml`
- Tagged releases via GoReleaser: `.github/workflows/release.yml`
- Automatic semantic tagging on `main`: `.github/workflows/auto-release.yml`

Local tooling:

```bash
make fmt
make vet
make test
make lint-workflows
make validate-workflows
```
