# Repository Guidelines

## Project Structure & Module Organization
- `cmd/bastion-session/`: Go binary entrypoint.
- `internal/app/`: Core domain logic (OCI client, runtime, context scope, tracking, state).
- `internal/cmd/`: Cobra command wiring and TUI entrypoints.
- `assets/`: systemd and launchd templates for background services.
- `README.md`: Build, usage, context scoping, and service setup.

## Build, Test, and Development Commands
- `go build -o bastion-session ./cmd/bastion-session`: Build CLI binary.
- `go test ./...`: Run automated test suite.
- `go run ./cmd/bastion-session --help`: Smoke-test CLI wiring.
- `go run ./cmd/bastion-session status`: Runtime smoke-test against OCI config.

## Coding Style & Naming Conventions
- Follow idiomatic Go formatting (`gofmt`) and package naming conventions.
- Keep command orchestration in `internal/cmd` and business logic in `internal/app`.
- Prefer small, composable functions and explicit error propagation.
- New CLI flags should have descriptive names and help text.

## Testing Guidelines
- Add tests as `*_test.go` near the package under test.
- Cover runtime behavior, config/path resolution, and OCI parsing edge cases.
- Use `t.TempDir()` for filesystem state and keep tests deterministic.
- Run `go test ./...` before opening PRs.

## Agent Contract
- Use JSON output for automation wherever the CLI supports it, including
  `bastion-session connect -o json`, `bastion-session ensure <host> -o json`,
  and `bastion-session session new <bastion-ref> -o json`.
- Treat JSON field names as stable contract. Prefer additive fields and document
  any breaking output change before relying on it in workflows or scripts.
- Preferred validation commands are `make fmt`, `make vet`, `make test`,
  `make lint-workflows`, and `make validate-workflows`.
- Release behavior is tag-driven: `v*` tags publish through GoReleaser, and the
  `auto-release` workflow can create semantic tags from Conventional Commit
  subjects on `main` while skipping commits that modify workflows.

## Demo Assets
- The README terminal capture is generated from `docs/demo/bastion-session.tape` with VHS.
- Keep the README focused on product usage; implementation details for regenerating the capture belong here.
- Use fictional examples in demo assets, currently `my-vps-01`, `my-bastion`, `cloud-user`, and `10.0.1.25`.
- After changing demo scripts or tapes, run:
  - `vhs validate docs/demo/bastion-session.tape`
  - `vhs docs/demo/bastion-session.tape`

## Commit & Pull Request Guidelines
- Follow Conventional Commits (`feat`, `fix`, `chore`, `refactor`).
- Keep commits atomic with imperative summaries.
- PRs should include intent, key changes, and test evidence.
- Include terminal snippets/screenshots for notable CLI/TUI behavior changes.

## Security & Configuration Tips
- Never commit OCI credentials, keys, or Terraform state.
- Prefer environment/flag-driven configuration; document new knobs in README.
- Ensure generated SSH fragments remain under `~/.ssh/config.d/bastion-session`.

## Python Deprecation
- The legacy Python package (`pyproject.toml`, `src/bastion_session_cli`, `tests/*.py`) is removed.
- `bastion-session` is now Go-only.
