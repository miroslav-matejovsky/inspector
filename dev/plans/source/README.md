# Source: collect observable signals into SQLite

## Goal

Build the first part of the toolkit named by the [Alignment](../../principles.md#alignment) principle: **Observe -> Source**.

- Package `source` reads configured HTTP endpoints of a running system on an interval. It stores every response, or every failed read, as a **signal** in an embedded SQLite database.
- The toolkit can write its runtime log to a file. Each run gets its own file.
- The workbench runs a Source against the inspected simulation. It writes its log to `logs/` and shows the collected signals as plain text on its page. The text view is temporary; the View part of the toolkit replaces it later.

## Scope

In scope:

- `internal/signalstore`: SQLite storage of signals: schema, insert, delete by age, per-target count and latest record. Driver `github.com/ncruces/go-sqlite3` v0.35.6: pure Go, no cgo.
- `internal/logfile`: one log file per run, written by a `log/slog` text handler.
- `source`: target configuration and validation, one collection round (`Collect`), the collection loop (`Run`), a per-target summary (`Summary`), and log lines when a target starts or stops failing.
- `cmd/workbench`: opens the log file and passes the logger to the workbench.
- `harness/workbench`: log and source flags; supervision of the Source next to the inspected app and the HTTP server; the text view at `/`.
- `go.mod`, `go.sum`, `vendor/`, `.go-arch-lint.yml`, `.gitignore`, `Taskfile.yml`, `README.md`, `harness/README.md`, `doc.go` files, `.todo`.

Out of scope:

- Parsing signal bodies (JSON, Prometheus text). Interpreting signals is the Model.
- Model and View packages.
- Sources other than HTTP GET (log files, event streams).
- Storing unchanged bodies once (deduplication), compression.
- Reading `/inspected/sim`. It is the harness operator's control API, not something the system exposes about itself.
- Any change to `harness/inspected`.
- Rotation or deletion of old log files.

## Boundary

```text
cmd/workbench -+-> internal/logfile                          creates logs/workbench-<time>.log
               '-> harness/workbench -+-> harness/inspected   serves /inspected/...
                                      '-> source -> internal/signalstore -> github.com/ncruces/go-sqlite3
                                            '-- HTTP GET only --> http://<workbench listener>/inspected/...
```

| Rule | Enforcement | Step |
| --- | --- | --- |
| `source` and `internal/*` never import `harness/...`. | `.go-arch-lint.yml`: `source` may depend only on `signalstore`; `signalstore` and `logfile` have no project dependencies. | 01 to 03 |
| Only `internal/signalstore` imports the SQLite driver. | `.go-arch-lint.yml`: vendor `sqlite` is listed in `canUse` of `signalstore` only. | 01 |
| Source only reads: HTTP GET without a request body. | `TestCollectUsesGet` checks the method and body length on the test server. | 03 |
| A failed read is stored as a signal. The log gets one line when a target starts failing and one when it recovers, not one per round. | `TestCollectStoresFailedRead`, `TestCollectLogsTransitionsOnce`. | 03 |
| Library tests use neutral fixtures: target names `alpha`, `beta`, paths `/a`, `/b`. | Rule of steps 01 to 03, checked in review. | 01 to 03 |

## Architecture impact

New packages:

| Package | Principle | Responsibility | May depend on |
| --- | --- | --- | --- |
| `source` | Observe | Collects signals from HTTP targets on an interval and stores them. | `internal/signalstore` |
| `internal/signalstore` | Observe (storage detail) | SQLite storage of signals. | stdlib, `github.com/ncruces/go-sqlite3` |
| `internal/logfile` | Proximity (cross-cutting detail) | One `slog` text log file per run. | stdlib |

Changed:

| Artifact | Change | Step |
| --- | --- | --- |
| `go.mod`, `go.sum`, `vendor/` | New direct dependency `github.com/ncruces/go-sqlite3 v0.35.6`. Indirect additions: `github.com/ncruces/go-sqlite3-wasm/v6 v6.3.35304`, `github.com/ncruces/julianday v1.0.0`. `golang.org/x/sys` moves from v0.47.0 to v0.48.0. `vendor/` grows by about 4.8 MB. | 01 |
| `.go-arch-lint.yml` | Vendor `sqlite`; components `signalstore`, `logfile`, `source`; `workbench` may depend on `source`. | 01, 02, 03, 05 |
| `cmd/workbench/main.go` | Opens the log file, prints its path, passes the logger to `workbench.Run`, closes the file. | 04 |
| `harness/workbench` | `Config.LogDir`, `Config.LogLevel`, `Config.Source`; `Run(ctx, cfg, logger)`; supervision of three goroutines; `Handler(inspected, signals, logger)`; `index.html` becomes a template with the text view. | 04, 05 |
| `Taskfile.yml` | `workbench` passes the log and source flags. | 04, 05 |
| `.gitignore` | `/logs/`, `/data/`. | 04, 05 |
| `README.md`, `harness/README.md`, `harness/workbench/doc.go`, `.todo` | Packages, flags, page and live verification. | 01 to 05 |

New workbench flags. Every flag is required, like the existing ones:

| Flag | Meaning | Value in `Taskfile.yml` | Step |
| --- | --- | --- | --- |
| `-log-dir` | Directory of the log files. Created when missing. | `logs` | 04 |
| `-log-level` | `debug`, `info`, `warn` or `error`. | `info` | 04 |
| `-source-database` | SQLite file of the collected signals. Its directory is created when missing. | `data/source.db` | 05 |
| `-source-interval` | Time between collection rounds. | `5s` | 05 |
| `-source-timeout` | Max wait for one read of one target. | `2s` | 05 |
| `-source-retention` | Signals older than this are deleted after each round. | `10m` | 05 |
| `-source-max-body-bytes` | A larger body is stored as a failed read. | `2097152` | 05 |
| `-source-target` | `name=path`, repeated. The path is on the workbench server. | 7 values, below | 05 |

Targets in `Taskfile.yml`: `index=/inspected/`, `health_live=/inspected/health/live`, `health_ready=/inspected/health/ready`, `metrics=/inspected/metrics`, `dependencies=/inspected/api/dependencies`, `products=/inspected/api/products`, `orders=/inspected/api/orders`.

Runtime files, ignored by git: `logs/workbench-<yyyyMMdd-HHmmss>.log` and `data/source.db` (with `-wal` and `-shm` files while it is open).

## Principles evaluation

| Principle | How this plan supports it |
| --- | --- |
| [Observe](../../principles.md#observe) | The Source takes what the system chooses to expose: the responses of its public GET endpoints (index, health, metrics, API). They are stored raw and unparsed, so nothing is lost before the Model exists. A read that fails is stored too: "the system did not answer" is an observation. The Source never calls the control API and never writes to the system. |
| [Navigate](../../principles.md#navigate) | Not implemented by this plan. Every signal keeps its target name, URL and time, so the Model can connect signals to entities later. `source` contains no relation logic. |
| [Explain](../../principles.md#explain) | Not implemented by this plan. The text view only proves that collection works. Explanation belongs to the View. |
| [Proximity](../../principles.md#proximity) | SQLite is embedded and pure Go: no database server, no C toolchain, one file in `data/` next to the code. The log file lands in `logs/` in the repository. `task workbench` starts everything. `task all` tests the storage without external services. |
| [Alignment](../../principles.md#alignment) | `source` is the Observe part and the only new top-level package. Storage and logging are implementation details in `internal/`; they do not add a fourth top-level concern. The temporary view stays in the harness until the View package exists. |

## Deliverables

- `internal/signalstore`: `doc.go`, `store.go`, `schema.go`, `store_test.go`.
- `internal/logfile`: `doc.go`, `logfile.go`, `logfile_test.go`.
- `source`: `doc.go`, `config.go`, `source.go`, `export_test.go`, `config_test.go`, `source_test.go`.
- `harness/workbench`: `signals.go`; updated `config.go`, `workbench.go`, `index.html`, `doc.go`, `config_test.go`, `workbench_test.go`.
- Updated `cmd/workbench/main.go`, `go.mod`, `go.sum`, `vendor/`, `.go-arch-lint.yml`, `.gitignore`, `Taskfile.yml`, `README.md`, `harness/README.md`, `.todo`.

## Success criteria

- `task all` exits 0. Its output contains `deadcode: no issues found` and `OK - No warnings found`.
- Every test named in steps 01 to 05 exists and passes.
- `go list -deps ./source ./internal/...` prints no package path containing `/harness/`.
- `git diff --stat` shows no change under `harness/inspected/`.
- `vendor/modules.txt` contains the line `# github.com/ncruces/go-sqlite3 v0.35.6`.
- `TestPageShowsCollectedSignals` (step 05) passes: a Source that collected once from the real inspected handler shows its targets with status `200` on `/`.
- `.todo` contains the section `## Source in the workbench`.

## Steps

| Step | Title | Depends on |
| --- | --- | --- |
| [01](01-signal-store.md) | Signal store and SQLite dependency | - |
| [02](02-log-file.md) | Log file | - |
| [03](03-source-package.md) | Source package | 01 |
| [04](04-workbench-logging.md) | Workbench logging | 02 |
| [05](05-workbench-source-and-text-view.md) | Workbench source and text view | 03, 04 |

Supporting documents: [alternatives.md](alternatives.md), [assessment.md](assessment.md), [progress.md](progress.md).
