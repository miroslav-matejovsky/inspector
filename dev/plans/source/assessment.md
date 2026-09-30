# Assessment

## Feasibility

High. The only new module is the SQLite driver. Everything else uses the standard library: `net/http`, `database/sql`, `log/slog`, `html/template`, `text/tabwriter`. The code follows patterns the repository already has: required flags without defaults, a supervisor in `workbench.Run`, handlers tested with `httptest`, an in-process inspected app in tests.

Checked while writing the plan, in scratch modules outside the repository:

- Toolchain: `go1.27.1 windows/amd64`, `task` 3.53.1, `golangci-lint` 2.14.0.
- `github.com/ncruces/go-sqlite3` v0.35.6 with `CGO_ENABLED=0` on Windows:
  - Driver name `sqlite3`.
  - The DSN `file:C:/.../s.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate` opens the file; `PRAGMA journal_mode` returns `wal`.
  - `PRAGMA user_version` reads 0 on a new file and can be set.
  - A `STRICT` table, inserts in a transaction and a query all work. `sqlite_version()` is `3.53.4`.
  - An empty `[]byte` body reads back as nil.
  - After `Close`, no `-wal` or `-shm` file is left and the directory can be removed. `ExecContext` after `Close` returns `sql: database is closed`.
- Vendor size in a fresh module: ncruces adds `github.com/ncruces` 4.8 MB; `golang.org/x/sys` is already vendored. `modernc.org/sqlite` v1.60.1 adds 123.2 MB.
- Cross-compile with `CGO_ENABLED=0` succeeds for `linux/amd64` and `darwin/arm64`. Cold build of the probe: 19 s, binary 13.6 MB.
- `go mod tidy` moves `golang.org/x/sys` from v0.47.0 to v0.48.0.
- Response sizes of the inspected service at seed 42 and 1 order per tick, measured with the real handler in process:

  | Path | Tick 0 | Tick 60 | Tick 2060 |
  | --- | ---: | ---: | ---: |
  | `/inspected/` | 363 | 363 | 363 |
  | `/inspected/health/live` | 16 | 16 | 16 |
  | `/inspected/health/ready` | 246 | 246 | 246 |
  | `/inspected/metrics` | 16329 | 23465 | 23607 |
  | `/inspected/api/dependencies` | 221 | 221 | 221 |
  | `/inspected/api/products` | 1005 | 1004 | 1004 |
  | `/inspected/api/orders` | 14 | 31739 | 533147 |

  `/api/orders` stops growing once the simulation keeps its maximum of 1000 terminal orders (`fulfillment.MaxRetainedOrders`). A full round is about 560 KB, so `-source-max-body-bytes 2097152` leaves room. Interval 5 s with retention 10 min holds about 120 rounds, about 67 MB.
- `slog.DiscardHandler`, `sync.WaitGroup.Go`, `slog.Level.UnmarshalText` and `flag.TextVar` exist in go1.27.1.
- golangci-lint v2 runs `errcheck` without the old default exclusions, so every `Close` error is handled.
- `deadcode ./cmd/...` sees only packages reachable from `cmd`. `internal/signalstore`, `internal/logfile` and `source` are invisible to it until steps 04 and 05 import them. Every exported function has a caller by the end of its step: `logfile` after step 04, `source` and `signalstore` after step 05. No allowlist entry is needed.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| ncruces is pre-1.0 and its API can change. | An upgrade breaks the build. | Version pinned in `go.mod` and vendored; only `internal/signalstore` imports it, through `database/sql`. |
| SQLite as Go code compiled from Wasm is slower than native SQLite. | Slow rounds under heavy load. | Workbench load is one transaction of 7 rows every 5 s. |
| The database grows faster than expected, for example with a higher `-inspected-orders-per-tick`. | Disk use grows. | Retention bounds it; the size follows from interval, retention and body size, documented in `alternatives.md`. SQLite reuses freed pages, so the file does not grow beyond its peak. |
| A body above `-source-max-body-bytes`. | The target shows an error instead of its body. | The error `body exceeds N bytes` is visible on the page; raise the flag. |
| A developer reads `data/source.db` with another SQLite client while the workbench runs. | A lock could block `Insert`, and the Source would stop the workbench. | WAL mode lets readers and the writer work at the same time. `busy_timeout(5000)` makes writes wait. |
| Windows cannot remove an open database file. | `t.TempDir` cleanup fails. | Tests close stores and sources in `t.Cleanup`, registered after `t.TempDir`, so they run first. |
| `data/source.db` from an older schema version. | `Open` fails at startup. | The error names the version and says to delete the file. This is the experimental phase: no migrations. |
| The Source reads the inspected service through the workbench listener. During shutdown its requests fail. | Failed-read signals at shutdown. | `ctx` is cancelled first, so the insert of that round fails and `Run` returns nil; no such signals are stored. |
| The log directory fills with one file per run. | Disk use grows slowly. | Files are small: start, stop and failure transitions only. Deleting them by hand is safe; `logs/` is ignored by git. |
| Two workbench runs start in the same second. | The second cannot create its log file. | `O_EXCL` makes it fail visibly with the path. |
| The page reloads while the user selects text. | The selection is lost. | Accepted for a temporary view. |

## Dependencies

- New module `github.com/ncruces/go-sqlite3` v0.35.6 and its indirect modules. Fetching them needs network access once; after that, `vendor/` holds them.
- No new tool; the tools used by `task all` do not change.
- Step order: 01 and 02 in any order; 03 after 01; 04 after 02; 05 after 03 and 04.
- Live verification (`.todo`) needs a browser and `curl`.

## Validation

- Each step: the `go test` command of the step, `go-arch-lint check`, `task all`.
- Library boundary: `go list -deps ./source ./internal/...` prints no `/harness/` path; only `internal/signalstore` imports the driver.
- Read-only toward the system: `TestCollectUsesGet`.
- Storage: `TestOpenReopensDatabase`, `TestDeleteBefore`, `TestCollectAppliesRetention`, `TestRunCreatesDatabase`.
- Failure handling: `TestCollectStoresFailedRead`, `TestCollectTimesOut`, `TestCollectRejectsLargeBody`, `TestRunReturnsStoreError`, `TestRunFailsWhenSourceCannotOpen`, `TestPageShowsSummaryError`.
- End to end in process: `TestPageShowsCollectedSignals`.
- Live behavior: `.todo` sections `## Workbench` and `## Source in the workbench`, done by hand outside `task all`.

## Rollback

The implementer commits nothing. To roll back the whole plan:

1. Delete `source/`, `internal/`, `harness/workbench/signals.go`, `logs/`, `data/`.
2. `git restore go.mod go.sum vendor .go-arch-lint.yml .gitignore Taskfile.yml README.md harness/README.md harness/workbench cmd/workbench .todo`.
3. `task all` passes with the previous state.

To roll back one step, revert the files listed under Target Artifacts of that step, in reverse step order. Step 01 includes `go.mod`, `go.sum` and `vendor/`.
