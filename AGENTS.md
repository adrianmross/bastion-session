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
