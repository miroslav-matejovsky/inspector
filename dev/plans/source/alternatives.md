# Alternatives

The decisions behind this plan, with the options that were rejected and why. The project owner decided decisions 1 and 3 (embedded SQLite, `internal/` for toolkit internals) in the request for this plan.

## 1. Database

| Option | Trade-off | Decision |
| --- | --- | --- |
| Embedded SQLite | One file, SQL queries for the later Model, no server. Replaceable later behind `internal/signalstore`. | Chosen |
| In-memory only | No dependency, but collected signals are lost on every restart. | Rejected |
| JSON or JSON Lines files | Standard library only, but every query is a full scan written by hand. Retention means rewriting files. | Rejected |
| bbolt | Pure Go key-value store. No SQL; secondary indexes by hand. | Rejected |

## 2. SQLite driver

| Option | Trade-off | Decision |
| --- | --- | --- |
| `github.com/ncruces/go-sqlite3` v0.35.6 | Pure Go (SQLite compiled from Wasm to Go), no cgo. Adds about 4.8 MB to `vendor/`. SQLite 3.53.4. Pre-1.0: the version is pinned and only one package imports it. | Chosen |
| `modernc.org/sqlite` v1.60.1 | Pure Go (SQLite translated from C), no cgo, 1.x. Adds about 123 MB to the committed `vendor/` (`modernc.org/libc` 66.9 MB, `modernc.org/sqlite` 55.8 MB). | Rejected: vendor size |
| `github.com/mattn/go-sqlite3` | The most used driver, but it needs cgo and a C compiler on Windows. Not fully embedded. | Rejected |

## 3. Where the storage code lives

| Option | Trade-off | Decision |
| --- | --- | --- |
| `internal/signalstore`, used only by `source` | The database is an implementation detail. Replacing it changes one package; the public `source` API stays. | Chosen |
| Unexported code inside `source` | Fewer packages, but SQL and HTTP collection mixed in one package, and replacing the database touches `source`. | Rejected |
| A public top-level storage package | Commits the toolkit to a storage API before the Model exists, and adds a top-level concern outside Observe, Model and View. | Rejected |

## 4. Types shared by `source` and `signalstore`

| Option | Trade-off | Decision |
| --- | --- | --- |
| Two types, `source.Signal` and `signalstore.Record`, copied field by field | About 20 lines of duplication. `signalstore` stays independent of the public API. | Chosen |
| `signalstore` imports `source.Signal` | `source` cannot then open the store itself (import cycle); the caller would have to wire it. | Rejected |
| A type alias of an internal type in `source` | One type, but the public API documents an internal package. | Rejected |

## 5. What a signal contains

| Option | Trade-off | Decision |
| --- | --- | --- |
| The raw response: status, content type, body, duration, or the error of a failed read | Domain-neutral. Keeps everything the system exposed for the later Model. | Chosen |
| Parsed values (JSON fields, Prometheus samples) | Interprets signals, which is the Model's job; ties the Source to formats before any are needed. | Rejected |

## 6. Failed reads

| Option | Trade-off | Decision |
| --- | --- | --- |
| Stored as signals with `Error`; a log record only when a target starts failing or recovers | A missing answer is an observation. The log stays short during an outage. | Chosen |
| Only logged | The history of a target has holes, and the log gets one line per target per round. | Rejected |

## 7. How a round reads its targets

| Option | Trade-off | Decision |
| --- | --- | --- |
| Concurrently, one goroutine per target, results in target order | A round takes at most `Timeout`. When the inspected simulation hangs, every target waits for its request timeout. | Chosen |
| One after another | Simpler, but a round can take the number of targets times `Timeout`. | Rejected |

## 8. Bounding the database

| Option | Trade-off | Decision |
| --- | --- | --- |
| Delete by age after every round (`-source-retention`) | One `DELETE` with an index. The size follows interval and retention: about 67 MB at 5 s and 10 min when `/api/orders` is at its largest. | Chosen |
| Keep N signals per target | Needs a window query per target; the time covered depends on the interval. | Rejected |
| No limit | About 560 KB per round at steady state; about 400 MB per hour at 5 s. | Rejected |
| Store a body only when it changed | Much smaller, but needs hashing and a signal that refers to an earlier body. | Rejected |

## 9. How targets are configured

| Option | Trade-off | Decision |
| --- | --- | --- |
| Repeated flag `-source-target name=path` in `Taskfile.yml` | Explicit. The Taskfile shows what is read. Same mechanism as the other required flags. | Chosen |
| A list in the workbench code | Hidden from the configuration. | Rejected |
| A new configuration file | A second configuration format for seven values. | Rejected |

## 10. Logging

| Option | Trade-off | Decision |
| --- | --- | --- |
| `internal/logfile`: `log/slog` text handler, one file per run named by start time | Standard library only. Readable, and `key=value` is machine-readable. A run's log is easy to find. | Chosen |
| `slog` JSON handler | Better for machines, harder to read while developing. | Rejected |
| One file for all runs, appended | Runs are mixed; the file grows without bound. | Rejected |
| A public top-level logging package | Adds a concern outside Observe, Model and View. Toolkit packages take a `*slog.Logger`, so any consumer can log where it wants. | Rejected |

## 11. Who owns the log file

| Option | Trade-off | Decision |
| --- | --- | --- |
| `cmd/workbench` opens and closes it; `workbench.Run` takes a `*slog.Logger` | The composition root owns resources. `Run` is tested with a discard or buffer logger. `main` prints the path. | Chosen |
| `workbench.Run` opens it from `Config.LogDir` | `Run` would create files in tests, and `main` cannot print the path. | Rejected |

## 12. Temporary visualization

| Option | Trade-off | Decision |
| --- | --- | --- |
| Server-rendered text table in `<pre>` on `/`, reloaded by a meta tag every 2 s, in `harness/workbench` | Few lines, tested with `httptest`, no JavaScript. Stays out of the toolkit until the View exists. | Chosen |
| A text/plain endpoint only | Works with `curl`, but the workbench page stays empty. | Rejected |
| JavaScript polling of a JSON endpoint | Client logic untested by `task all`. | Rejected |
| A first `view` package in the toolkit | Designs the View before the Model exists. | Rejected |

## 13. Deterministic time in tests

| Option | Trade-off | Decision |
| --- | --- | --- |
| Unexported clock field, set by `SetClock` in `export_test.go` | Tests of retention and signal times are exact. The public API stays unchanged. | Chosen |
| A clock in `Config` | Every caller would pass a clock only tests need. | Rejected |
| Real time | Retention tests would depend on timer resolution and sleeps. | Rejected |
