# Development

## Requirements

- Go 1.27.1 or newer.
- [Task](https://taskfile.dev) to run the tasks of [Taskfile.yml](../Taskfile.yml).
- PowerShell 7 (`pwsh`), used by the scripts in `taskfile/`.
- golangci-lint.
- The Go tools installed by `task go-tools`: deadcode, gotestsum, go-arch-lint and govulncheck.

Dependencies are vendored in `vendor/`; nothing in it is edited by hand.

## Tasks

| Task | Purpose |
| --- | --- |
| `task all` | Every check and the tests: clean, tidy, fmt, vet, deadcode, arch-lint, lint, test. Work is complete only when it passes. |
| `task test` | Unit tests; output is saved to `.test-results/`. |
| `task workbench` | Runs the development workbench on http://localhost:8080. |
| `task go-tools` | Installs or updates the required Go tools. |
| `task deps` | Lists direct dependencies with available updates. |
| `task vendor` | Re-syncs `vendor/` after changing dependencies. |
| `task release -- <version>` | Tags and pushes a module release from `main`. |

`task` without arguments lists every task.

## Checks

- `deadcode` fails on functions that no command in `cmd/` reaches. The allowlist in [taskfile/deadcode.ps1](../taskfile/deadcode.ps1) is for staged, sealing and test-support functions only.
- `go-arch-lint` fails on imports that break the rules of [architecture.md](architecture.md).

## Repository layout

| Path | Contents |
| --- | --- |
| `source/`, `model/`, `view/` | The toolkit, see [architecture.md](architecture.md). |
| `internal/` | Implementation details of the toolkit. |
| `harness/` | Development workbench and simulated service, see [harness/README.md](../harness/README.md). |
| `cmd/workbench/` | Entry point of the workbench. |
| `dev/` | Working records: principles, assessments, backlog, bugs, evaluations, investigations and plans, see [dev/README.md](../dev/README.md). |
| `docs/` | This documentation. |
| `.todo` | Actionable items outside the working records, such as live verification. |
