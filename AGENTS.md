# Repository Guidelines

## Project Structure & Module Organization
- `src/bastion_session_cli/`: CLI entrypoints, runtime orchestration, and OCI integrations.
- `tests/`: Pytest suite covering config parsing, session cache, and runtime flows.
- `assets/`: systemd and launchd templates for background services.
- `README.md`: Installation, usage, and background service guidance.

## Build, Test, and Development Commands
- `python -m venv .venv && source .venv/bin/activate`: Create and enter an isolated environment.
- `pip install -e .`: Install the CLI in editable mode.
- `pytest`: Run the automated test suite.
- `bastion-session status`: Smoke-test the CLI against configured OCI credentials.

## Coding Style & Naming Conventions
- Target Python 3.10+ with 4-space indentation and snake_case identifiers.
- Keep module boundaries focused: CLI in `main.py`, orchestration in `runtime.py`, OCI calls in `oci_client.py`.
- Preserve type hints and `Path` usage; prefer `rich` for formatted console output.
- Introduce new Click options with descriptive names and help strings.

## Testing Guidelines
- Add tests under `tests/` mirroring module names (e.g., `test_runtime.py`).
- Cover new CLI flags, cache logic, and OCI interactions with parametrized cases.
- Use temporary directory fixtures for filesystem operations and clean up SSH fragments.
- Run `pytest` before PRs and capture output in the PR description.

## Commit & Pull Request Guidelines
- Follow Conventional Commits (`feat`, `fix`, `chore`, `refactor`) as seen in history.
- Keep commits atomic with imperative summaries; explain configuration or flag changes in the body.
- PRs should outline intent, notable changes, test evidence, and linked issues.
- Include screenshots or snippets for terminal output or service manifest updates.

## Security & Configuration Tips
- Never commit OCI credentials, keys, or Terraform state; rely on environment variables.
- Document new configuration knobs in `Config.from_env()` docstrings and README updates.
- Confirm generated SSH fragments stay under `~/.ssh/config.d/bastion-session` before merging.
