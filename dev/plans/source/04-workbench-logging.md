---
title: "04 - Workbench logging"
dependencies: ["02"]
effort: "S"
complexity: "low"
---

# 04 - Workbench logging

## Objective

`task workbench` writes the runtime log of each run to `logs/workbench-<yyyyMMdd-HHmmss>.log`. `cmd/workbench` opens the file with `internal/logfile` and passes its logger to `workbench.Run`. `Run` logs when the workbench starts and stops. Two new required flags, `-log-dir` and `-log-level`, configure the file.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/config.go` | Fields `LogDir`, `LogLevel`; flags `-log-dir`, `-log-level`; `Validate` checks `LogDir`. |
| `harness/workbench/workbench.go` | `Run(ctx, cfg, logger)`; logs `workbench started` and `workbench stopped`. |
| `cmd/workbench/main.go` | `main` calls `run() int`; `run` opens the log file, prints its path, runs the workbench, closes the file. |
| `harness/workbench/config_test.go` | New flags in `allFlags`, expectations and invalid values. |
| `harness/workbench/workbench_test.go` | `Run` calls pass a logger; new tests below. |
| `harness/workbench/doc.go` | Flags and a `# Logging` section. |
| `Taskfile.yml` | `workbench` passes `-log-dir logs -log-level info`; the command becomes a folded block. |
| `.gitignore` | `/logs/`. |
| `harness/README.md`, `README.md` | Flags and the log location. |
| `.todo` | Live checks of the log file in section `## Workbench`. |

## Implementation Tasks

1. Update `config_test.go` and `workbench_test.go` as listed below. Confirm they fail.
2. Update `config.go`, then `workbench.go`.
3. Update `cmd/workbench/main.go`.
4. Update `Taskfile.yml` and `.gitignore`.
5. Update `doc.go`, `harness/README.md`, `README.md` and `.todo`.
6. Run `go test ./harness/workbench/...` until it passes, then `task all`.

## Technical Details

### `config.go`

```go
type Config struct {
    Addr      string     // listen address, for example "localhost:8080"
    LogDir    string     // directory of the log files, used by cmd/workbench
    LogLevel  slog.Level // lowest level written to the log file, used by cmd/workbench
    Inspected inspected.Config
}
```

- `Validate`: `c.LogDir == ""` gives `workbench: log dir is required`. It is checked after `Addr`.
- `ParseConfig`:
  - `fs.StringVar(&cfg.LogDir, "log-dir", "", "directory of the log files, created when missing, for example logs (required)")`
  - `fs.TextVar(&cfg.LogLevel, "log-level", slog.LevelInfo, "lowest level written to the log file: debug, info, warn or error (required)")`
  - The existing required-flag check covers both, because it lists every flag that was not set.

### `workbench.go`

```go
// Run validates cfg, starts the inspected app and the HTTP server and blocks
// until ctx is cancelled or one of them fails. logger receives the start and
// stop records of the workbench.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error
```

- `logger == nil` gives `workbench: logger is required`. It is checked before `cfg.Validate()`.
- After `net.Listen` succeeds: `logger.Info("workbench started", "addr", ln.Addr().String())`.
- Before returning, after both goroutines have stopped, with `err := errors.Join(errs...)`:
  - `err == nil`: `logger.Info("workbench stopped")`
  - otherwise: `logger.Error("workbench stopped", "error", err)`
- Failures before `net.Listen` succeeds are returned without a log record; `cmd/workbench` prints them to stderr.

### `cmd/workbench/main.go`

```go
func main() { os.Exit(run()) }

// run returns the exit code: 2 for invalid flags, 1 for a failure, 0 after a
// clean shutdown. Deferred calls run before os.Exit because they belong to run.
func run() int {
    cfg, err := workbench.ParseConfig(os.Args[1:], os.Stderr)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        return 2
    }
    logFile, err := logfile.Open(cfg.LogDir, "workbench", cfg.LogLevel, time.Now())
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        return 1
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    fmt.Printf("workbench listening on http://%s (inspected at http://%s%s/), log %s\n",
        cfg.Addr, cfg.Addr, inspected.PathPrefix, logFile.Path())
    code := 0
    if err := workbench.Run(ctx, cfg, logFile.Logger()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        code = 1
    }
    if err := logFile.Close(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        code = 1
    }
    return code
}
```

`main` has no tests. It only composes tested parts: `ParseConfig`, `logfile.Open` and `Run`. The live check in `.todo` covers it.

### `Taskfile.yml`

```yaml
  workbench:
    desc: Run the development workbench with the inspected simulation on localhost:8080, logging to logs/
    silent: true
    cmds:
      - >-
        go run ./cmd/workbench
        -addr localhost:8080
        -log-dir logs
        -log-level info
        -inspected-seed 42
        -inspected-orders-per-tick 1
        -inspected-tick-interval 1s
        -inspected-request-timeout 2s
```

`>-` folds the lines into one command line separated by spaces.

### `.gitignore`

Append `/logs/`.

### Documentation

- `harness/workbench/doc.go`: add both flags to the `# Configuration` list; new section `# Logging`: `Run` writes `workbench started` with the listen address and `workbench stopped` with the error, if any; `cmd/workbench` owns the log file.
- `harness/README.md`: rows `-log-dir` and `-log-level` in the flag table. A sentence: each run writes `logs/workbench-<yyyyMMdd-HHmmss>.log`; the path is printed at startup.
- `README.md`: add to the `## Harness` paragraph: `Each run writes its log to logs/.`
- `.todo`, section `## Workbench`, add:
  - `After task workbench, logs/ holds a new workbench-<time>.log whose first line contains msg="workbench started" and the address.`
  - `After Ctrl+C, the last line of that file contains msg="workbench stopped" and no error.`

### Tests

`config_test.go`:

| Test | Change or assertion |
| --- | --- |
| `allFlags` | Add `{"log-dir", "logs"}` and `{"log-level", "info"}`. |
| `TestParseConfig` | Expected config adds `LogDir: "logs"`, `LogLevel: slog.LevelInfo`. |
| `TestParseConfigWithoutFlags` | The message contains `missing required flags: -addr, -inspected-orders-per-tick, -inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -log-dir, -log-level`. |
| `TestParseConfigRejectsInvalidValues` | New cases: `empty log dir` `{"log-dir": ""}`; `unknown log level` `{"log-level": "loud"}`. |
| `TestParseConfigAcceptsLogLevels` | New. Table `debug`, `WARN`, `error` give `slog.LevelDebug`, `slog.LevelWarn`, `slog.LevelError`. |

`workbench_test.go`:

| Test | Change or assertion |
| --- | --- |
| `validConfig` | Sets `LogDir: "logs"`. `Run` does not use it, so no directory is created. |
| existing `Run` tests | Pass `slog.New(slog.DiscardHandler)`. |
| `TestRunRejectsNilLogger` | New. `Run(ctx, validConfig(), nil)` returns an error containing `logger is required`. |
| `TestRunLogsStartAndStop` | New. Logger writes to a `bytes.Buffer` through `slog.NewTextHandler`. `ctx` is cancelled before `Run`. `Run` returns nil. The buffer contains `msg="workbench started"` followed by `addr=127.0.0.1:`, and then `msg="workbench stopped"` without `error=`. The buffer is read only after `Run` returned. |

## Verification

```powershell
go test ./harness/workbench/...
go build ./cmd/workbench
task all
```

## Acceptance Criteria

- `go test ./harness/workbench/...` exits 0 and runs every test listed above.
- `task all` exits 0, and its `deadcode` step prints `deadcode: no issues found`.
- `Taskfile.yml` task `workbench` contains `-log-dir logs` and `-log-level info`.
- `.gitignore` contains the line `/logs/`.
- `harness/README.md` and `harness/workbench/doc.go` list `-log-dir` and `-log-level`.

## Non-Goals

- Source flags and supervision (step 05).
- Logging of HTTP requests.
- Log records of the inspected simulation.
- Writing the log to stderr as well.
