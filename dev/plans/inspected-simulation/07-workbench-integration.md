---
title: "07 - Workbench integration and documentation"
dependencies: ["06-business-api"]
effort: "S"
complexity: "low"
---

# 07 - Workbench integration and documentation

## Objective

The workbench starts the inspected app in-process, mounts it under `/inspected/` on its existing HTTP server and supervises it: a failure of either the HTTP server or the inspected app stops the workbench with an error. All configuration comes from required command-line flags. `task workbench` passes every flag explicitly. Documentation and `.todo` describe the result.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/config.go` | New: `Config`, `Config.Validate`, `ParseConfig`. |
| `harness/workbench/workbench.go` | `Handler(inspected http.Handler)`, `Run(ctx, cfg Config)`. |
| `harness/workbench/doc.go` | Describes mounting, flags, supervision. |
| `harness/workbench/config_test.go` | New tests. |
| `harness/workbench/workbench_test.go` | Tests adapted and added. |
| `cmd/workbench/main.go` | Uses `ParseConfig`. |
| `Taskfile.yml` | `workbench` task passes all flags. |
| `.go-arch-lint.yml` | `workbench: { mayDependOn: [inspected] }`. |
| `harness/README.md` | Inspected section: domain, endpoints, flags, scenarios. |
| `README.md` (root) | Harness section. |
| `.todo` | Live verification section `Inspected simulation`. |
| `dev/plans/inspected-simulation/progress.md` | All steps marked done. |

`harness/workbench/index.html` is not changed.

## Implementation Tasks

1. Write `config_test.go` and update `workbench_test.go` with every test listed below. Confirm they fail.
2. Create `config.go`.
3. Change `Handler` and `Run` in `workbench.go`.
4. Change `cmd/workbench/main.go`.
5. Change the `workbench` task in `Taskfile.yml`.
6. Add the `workbench` rule to `.go-arch-lint.yml`.
7. Update `harness/workbench/doc.go`, `harness/README.md`, root `README.md`, `.todo`.
8. Run `task all`. If `task deadcode` reports a function, the function is not used by the application: remove it or wire it. Do not add allowlist entries.
9. Mark all steps `done` in `progress.md`.

## Technical Details

### Config (`config.go`)

```go
// Config configures the workbench. Every field is required.
type Config struct {
    Addr      string          // listen address, not empty
    Inspected inspected.Config
}

func (c Config) Validate() error // Addr not empty, then c.Inspected.Validate()

// ParseConfig parses command-line arguments. Every flag is required; a missing
// flag is an error even if its zero value would be valid. Usage and parse
// errors are written to output.
func ParseConfig(args []string, output io.Writer) (Config, error)
```

`ParseConfig`:

1. `fs := flag.NewFlagSet("workbench", flag.ContinueOnError)`, `fs.SetOutput(output)`.
2. Define flags: `addr` (string), `inspected-seed` (uint64), `inspected-orders-per-tick` (int), `inspected-tick-interval` (duration), `inspected-request-timeout` (duration). Zero values in code; the flag help text of each says `(required)`.
3. `fs.Parse(args)`; wrap error as `workbench: parse flags: %w`.
4. Collect set flags with `fs.Visit`; collect missing ones with `fs.VisitAll`. If any is missing, return `workbench: missing required flags: -a, -b` in lexicographical order (the order `fs.VisitAll` uses).
5. Build `Config`, return `cfg, cfg.Validate()`.

### Handler

```go
// Handler serves the workbench page at "/" and delegates every path under
// inspected.PathPrefix+"/" to the inspected handler without stripping the prefix.
func Handler(inspectedHandler http.Handler) http.Handler
```

Adds `mux.Handle(inspected.PathPrefix+"/", inspectedHandler)` to the existing mux.

### Run and supervision

```go
// Run validates cfg, starts the inspected app and the HTTP server, and blocks
// until ctx is cancelled or one of them fails. It is the supervisor of both:
// any unexpected stop cancels the other and is returned as an error.
// Restart is not attempted; the process exits and a new start rebuilds the
// inspected state from the seed.
func Run(ctx context.Context, cfg Config) error
```

Sequence:

1. `cfg.Validate()`; `app, err := inspected.New(cfg.Inspected)`.
2. `ln, err := net.Listen("tcp", cfg.Addr)`.
3. `runCtx, cancel := context.WithCancel(ctx)`; `defer cancel()`.
4. `appDone := make(chan error, 1)`; `go func() { appDone <- app.Run(runCtx) }()`.
5. `srv := &http.Server{Handler: Handler(app.Handler()), ReadHeaderTimeout: 5 * time.Second}`; `serveDone := make(chan error, 1)`; `go func() { serveDone <- srv.Serve(ln) }()`.
6. `select`:
   - `<-ctx.Done()`: no failure.
   - `err := <-appDone`: failure `workbench: inspected stopped: %w` (a nil `err` becomes `errors.New("unexpected stop")`); mark `appDone` as consumed.
   - `err := <-serveDone`: failure `workbench: serve: %w`.
7. `cancel()`; `srv.Shutdown` with a 5 second timeout detached from `ctx` (existing code); wait for `appDone` unless consumed.
8. Return the failure, else the shutdown error, else nil.

Both goroutines have an owner (`Run`) and a termination path (`runCtx` cancel, `srv.Shutdown`).

### Entry point (`cmd/workbench/main.go`)

```go
cfg, err := workbench.ParseConfig(os.Args[1:], os.Stderr)
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(2)
}
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
fmt.Printf("workbench listening on http://%s (inspected at http://%s%s/)\n", cfg.Addr, cfg.Addr, inspected.PathPrefix)
if err := workbench.Run(ctx, cfg); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

### Taskfile

```yaml
  workbench:
    desc: Run the development workbench with the inspected simulation on localhost:8080
    silent: true
    cmds:
      - go run ./cmd/workbench -addr localhost:8080 -inspected-seed 42 -inspected-orders-per-tick 1 -inspected-tick-interval 1s -inspected-request-timeout 2s
```

### Documentation

- `harness/workbench/doc.go`: page at `/`, inspected mounted at `/inspected/`, flags, supervision and restart behavior.
- `harness/README.md`: replace the `inspected` paragraph with: purpose, simulated domain (products, orders, dependencies, ticks), endpoint table (copy of the plan README table without the Step column), flag table, and three reproducible manual scenarios with exact requests: payment gateway outage, slow payment gateway, stock depletion and restock.
- Root `README.md`: add a `## Harness` section with 2 to 3 sentences and a link to `harness/README.md`.

### `.todo` entry

```markdown
## Inspected simulation

- Run `task workbench`, open http://localhost:8080/inspected/ and follow every link; each returns 200.
- Pipe `curl -s http://localhost:8080/inspected/metrics` into `promtool check metrics`; no errors.
- Add `localhost:8080` with `metrics_path: /inspected/metrics` to a local Prometheus scrape config; target is `UP` and `fulfillment_simulation_tick` increases by 1 per second.
- `curl -X PUT -d '{"mode":"outage"}' http://localhost:8080/inspected/sim/dependencies/warehouse`, wait 5 seconds: `/inspected/health/ready` returns 503 and new simulated orders fail with `warehouse_unavailable`. Set `healthy`: readiness returns 200 again.
- Press Ctrl+C: the process exits with code 0 within 5 seconds.
```

### Tests

`config_test.go`:

| Test | Assertion |
| --- | --- |
| `TestParseConfig` | Args `-addr localhost:8080 -inspected-seed 42 -inspected-orders-per-tick 1 -inspected-tick-interval 1s -inspected-request-timeout 2s` give `Config{Addr: "localhost:8080", Inspected: {Seed: 42, OrdersPerTick: 1, TickInterval: time.Second, RequestTimeout: 2 * time.Second}}`. |
| `TestParseConfigWithoutFlags` | Empty args give an error containing `missing required flags: -addr, -inspected-orders-per-tick, -inspected-request-timeout, -inspected-seed, -inspected-tick-interval`. |
| `TestParseConfigRequiresEveryFlag` | Table over the 5 flags: omitting one returns an error whose message contains `-<flag name>`. |
| `TestParseConfigAcceptsZeroSeedAndTraffic` | `-inspected-seed 0 -inspected-orders-per-tick 0` with the other flags set succeeds. |
| `TestParseConfigRejectsInvalidValues` | Table: `-addr ""`, `-inspected-tick-interval 0s`, `-inspected-request-timeout 0s`, `-inspected-orders-per-tick 101`, `-inspected-seed -1`, `-inspected-tick-interval abc` each return an error. |

`workbench_test.go`:

| Test | Assertion |
| --- | --- |
| `TestHandlerServesTwoPanels` | Existing assertions, handler built as `Handler(stub)`. |
| `TestHandlerMountsInspected` | Stub records `r.URL.Path`; `GET /inspected/api/products` reaches the stub with path `/inspected/api/products`. |
| `TestHandlerUnknownPathIsNotFound` | Existing: `GET /missing` is 404; stub is not called. |
| `TestWorkbenchServesInspected` | Real `inspected.New` with a test config (`TickInterval: time.Hour`), `app.Run` in a goroutine stopped on cleanup; `Handler(app.Handler())` answers `GET /inspected/health/live` with 200. |
| `TestRunRejectsInvalidConfig` | `Run(ctx, Config{})` returns an error. |
| `TestRunStopsOnContextCancel` | Valid config with `Addr: "127.0.0.1:0"`; cancel; `Run` returns nil within 5 seconds. |

## Verification

```powershell
go test ./harness/... ./cmd/...
task deadcode
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/... ./cmd/...` exits 0 and runs every test listed above.
- `task deadcode` prints `deadcode: no issues found (allowlisted symbols skipped)` and `taskfile/deadcode.ps1` has an empty `$allow` list.
- `TestParseConfigWithoutFlags` passes: empty args give an error containing `missing required flags` and all 5 flag names.
- `Taskfile.yml` task `workbench` passes all 5 flags.
- `harness/README.md` contains the endpoint table and the three manual scenarios.
- `.todo` contains the section `## Inspected simulation`.
- `git diff --stat` shows no change to `harness/workbench/index.html`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Workbench UI changes. The inspected and inspector panels stay empty.
- Starting the server in tests on a fixed port or verifying against a running process (recorded in `.todo`).
- Restarting the inspected app after a failure.
- Configuration files. Flags are the only configuration source.
