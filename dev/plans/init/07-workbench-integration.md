---
title: "07 - Workbench integration and documentation"
dependencies: ["05-harness-adapter", "06-representation-json-views"]
effort: "S"
complexity: "low"
---

# 07 - Workbench integration and documentation

## Objective

The workbench consumes the Inspector library against the inspected simulation. It builds the inspector views from `connectivity.HTTPReader`, `harness/adapter` and `representation.NewHandler`, and mounts them under `/inspector/` on its existing HTTP server next to `/inspected/`. The inspector reads the inspected service over HTTP through the workbench's own listener, as an external client would. One new required flag bounds each read. Documentation and `.todo` describe the result.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/config.go` | Field `Config.InspectorSourceTimeout`, validation, flag `-inspector-source-timeout`. |
| `harness/workbench/workbench.go` | `InspectorPathPrefix`, `InspectorHandler`, `Handler(inspectedHandler, inspectorHandler)`, `Run` builds and mounts the inspector. |
| `harness/workbench/doc.go` | Sections on the inspector mount and the new flag. |
| `harness/workbench/config_test.go` | Tests adapted. |
| `harness/workbench/workbench_test.go` | Tests adapted and added. |
| `cmd/workbench/main.go` | Prints the inspector URL. |
| `Taskfile.yml` | Task `workbench` passes `-inspector-source-timeout 2s`. |
| `.go-arch-lint.yml` | `workbench: { mayDependOn: [inspected, adapter, connectivity, representation] }`. |
| `harness/README.md` | Workbench flag row, inspector endpoint table. |
| `README.md` (root) | Harness section mentions the inspector views. |
| `.todo` | Live verification section `## Inspector in the workbench`. |
| `dev/plans/init/progress.md` | Step 07 marked `done`. |

`harness/workbench/index.html` and everything under `harness/inspected/` stay unchanged.

## Implementation Tasks

1. Adapt `config_test.go` and `workbench_test.go` as listed below. Confirm they fail.
2. Change `config.go`.
3. Change `workbench.go`.
4. Change `cmd/workbench/main.go`.
5. Change the `workbench` task in `Taskfile.yml`.
6. Change the `workbench` rule in `.go-arch-lint.yml`.
7. Update `harness/workbench/doc.go`, `harness/README.md`, root `README.md`, `.todo`.
8. Run `task all`. If `task deadcode` reports a function, it is not used by the application: remove it or wire it. Do not add allowlist entries.
9. Mark step 07 `done` in `progress.md`.

## Technical Details

### Config (`config.go`)

```go
type Config struct {
    Addr                   string          // listen address, for example "localhost:8080"
    Inspected              inspected.Config
    InspectorSourceTimeout time.Duration   // max wait for one read of the inspected service by the inspector, must be positive
}
```

`Validate`: after the `Addr` check, `InspectorSourceTimeout <= 0` returns `workbench: inspector source timeout must be positive, got %s`; then `c.Inspected.Validate()`.

`ParseConfig` adds:

```go
fs.DurationVar(&cfg.InspectorSourceTimeout, "inspector-source-timeout", 0,
    "max wait for one read of the inspected service by the inspector, for example 2s (required)")
```

### Handlers (`workbench.go`)

```go
// InspectorPathPrefix isolates every inspector endpoint. Handler mounts the
// inspector handler at InspectorPathPrefix + "/"; its routes include the prefix.
const InspectorPathPrefix = "/inspector"

// Handler serves the workbench page at "/", delegates every path under
// inspected.PathPrefix+"/" to inspectedHandler and every path under
// InspectorPathPrefix+"/" to inspectorHandler, without stripping prefixes.
func Handler(inspectedHandler, inspectorHandler http.Handler) http.Handler

// InspectorHandler builds the Inspector views of the inspected service at
// inspectedURL, for example "http://127.0.0.1:8080/inspected". Every view
// request reads the service with GET requests, each bounded by sourceTimeout.
func InspectorHandler(inspectedURL string, sourceTimeout time.Duration) (http.Handler, error)
```

`InspectorHandler`:

```go
reader, err := connectivity.NewHTTPReader(inspectedURL, sourceTimeout, &http.Client{})
if err != nil {
    return nil, fmt.Errorf("workbench: inspector: %w", err)
}
source, err := adapter.New(reader, time.Now)
if err != nil {
    return nil, fmt.Errorf("workbench: inspector: %w", err)
}
h, err := representation.NewHandler(InspectorPathPrefix, source)
if err != nil {
    return nil, fmt.Errorf("workbench: inspector: %w", err)
}
return h, nil
```

`&http.Client{}` has no client timeout on purpose: `HTTPReader` bounds each request with a context timeout.

### Run

Directly after `net.Listen` succeeds:

```go
inspectorHandler, err := InspectorHandler("http://"+ln.Addr().String()+inspected.PathPrefix, cfg.InspectorSourceTimeout)
if err != nil {
    if closeErr := ln.Close(); closeErr != nil {
        err = errors.Join(err, fmt.Errorf("workbench: close listener: %w", closeErr))
    }
    return err
}
```

The server handler becomes `Handler(app.Handler(), inspectorHandler)`. Supervision and shutdown are unchanged. `ln.Addr()` is used instead of `cfg.Addr` because it holds the real port when `cfg.Addr` ends in `:0`.

### Production callers (for `task deadcode`)

| Function | Production caller |
| --- | --- |
| `observation.NewSnapshot` | `adapter.Adapter.Observe` |
| `observation.Snapshot.ObservedAt`, `Entities`, `Entity` | `representation` view handlers |
| `observation.Snapshot.Relations` | `navigation.Links` |
| `observation.Ref.String` | error messages of `NewSnapshot`, `navigation.Links`, `representation` not-found message |
| `navigation.Links` | `representation` entity detail |
| `connectivity.NewHTTPReader` | `workbench.InspectorHandler` |
| `connectivity.HTTPReader.Get`, `connectivity.Document.Decode` | `adapter.Adapter.Observe` |
| `adapter.New` | `workbench.InspectorHandler` |
| `adapter.Adapter.Observe` | `representation` view handlers, through `representation.Observer` |
| `representation.NewHandler` | `workbench.InspectorHandler` |
| `workbench.InspectorHandler` | `workbench.Run` |

### Entry point (`cmd/workbench/main.go`)

```go
fmt.Printf("workbench listening on http://%s (inspected at http://%s%s/, inspector at http://%s%s/)\n",
    cfg.Addr, cfg.Addr, inspected.PathPrefix, cfg.Addr, workbench.InspectorPathPrefix)
```

### Taskfile

```yaml
  workbench:
    desc: Run the development workbench with the inspected simulation and the inspector on localhost:8080
    silent: true
    cmds:
      - go run ./cmd/workbench -addr localhost:8080 -inspected-seed 42 -inspected-orders-per-tick 1 -inspected-tick-interval 1s -inspected-request-timeout 2s -inspector-source-timeout 2s
```

### Documentation

- `harness/workbench/doc.go`: new section `# Inspector`: mount at `InspectorPathPrefix + "/"`, composition `connectivity.HTTPReader` -> `adapter` -> `representation`, reads over HTTP through the workbench listener. Flag list gains `-inspector-source-timeout`.
- `harness/README.md`, section `## workbench`: add the flag row `| -inspector-source-timeout | Max wait for one read of the inspected service by the inspector, for example 2s. |`, a sentence that the Inspector views are mounted under `/inspector/`, and the inspector endpoint table from the plan `README.md`.
- Root `README.md`, section `## Harness`: add `The workbench also serves the Inspector views of the simulation under /inspector/.`

### `.todo` entry

```markdown
## Inspector in the workbench

- Run `task workbench`, open http://localhost:8080/inspector/: `kinds` lists `service` (1), `health_check` (3), `product` (5) and, after the first ticks, `order`.
- Follow `entities_href`, then the `href` of an order: `related` contains `for_product` to its product. The product's `related` lists the order as `incoming`.
- `curl -X PUT -d '{"mode":"outage"}' http://localhost:8080/inspected/sim/dependencies/warehouse`: `/inspector/entities/service/inspected` shows state `down`; `/inspector/entities/health_check/warehouse` shows state `down` and attribute `reason`. Set `healthy` again.
- Press Ctrl+C: the process exits with code 0 within 5 seconds.
```

### Tests

`config_test.go`:

| Test | Change |
| --- | --- |
| `allFlags` | Adds `{"inspector-source-timeout", "2s"}`. |
| `TestParseConfig` | Expects `InspectorSourceTimeout: 2 * time.Second`. |
| `TestParseConfigWithoutFlags` | Expects `missing required flags: -addr, -inspected-orders-per-tick, -inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -inspector-source-timeout`. |
| `TestParseConfigRequiresEveryFlag` | Unchanged; covers the new flag through `allFlags`. |
| `TestParseConfigRejectsInvalidValues` | Adds cases `zero source timeout` (`0s`), `negative source timeout` (`-1s`), `unparsable source timeout` (`x`). |

`workbench_test.go`:

| Test | Assertion |
| --- | --- |
| `validConfig` | Adds `InspectorSourceTimeout: time.Second`. |
| `startInspected(t)` | New helper: `inspected.New(validConfig().Inspected)`, `app.Run` in a goroutine, cancelled on cleanup, must return nil. Used by the two tests below that need a real app. |
| `TestHandlerServesTwoPanels` | Existing assertions with `Handler(&stubHandler{}, &stubHandler{})`. |
| `TestHandlerMountsInspected` | `GET /inspected/api/products` reaches the inspected stub with that path; the inspector stub is not called. |
| `TestHandlerMountsInspector` | New: `GET /inspector/entities` reaches the inspector stub with path `/inspector/entities`; the inspected stub is not called. |
| `TestHandlerUnknownPathIsNotFound` | `GET /missing` is 404; neither stub is called. |
| `TestWorkbenchServesInspected` | Existing, uses `startInspected` and `Handler(app.Handler(), &stubHandler{})`. |
| `TestInspectorHandlerObservesInspected` | New: `app := startInspected(t)`; `srv := httptest.NewServer(app.Handler())`, closed on cleanup; `h, err := workbench.InspectorHandler(srv.URL+inspected.PathPrefix, time.Second)`; `GET /inspector/` through `workbench.Handler(app.Handler(), h)` is 200; decoded `kinds` have `kind` and `count` equal to `[service 1, health_check 3, product 5]`. |
| `TestInspectorHandlerRejectsInvalidConfig` | New: `InspectorHandler("", time.Second)` and `InspectorHandler("http://127.0.0.1:1/inspected", 0)` return errors. |
| `TestRunRejectsInvalidConfig`, `TestRunFailsWhenAddressIsTaken`, `TestRunStopsOnContextCancel` | Unchanged; `validConfig` carries the new field. `TestRunStopsOnContextCancel` covers building the inspector inside `Run`. |

## Verification

```powershell
go test ./harness/... ./cmd/...
task deadcode
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./harness/... ./cmd/...` exits 0 and runs every test listed above.
- `TestInspectorHandlerObservesInspected` passes.
- `task deadcode` prints `deadcode: no issues found (allowlisted symbols skipped)` and `taskfile/deadcode.ps1` has an empty `$allow` list.
- `Taskfile.yml` task `workbench` passes all 6 flags.
- `.go-arch-lint.yml` rule for `workbench` is exactly `mayDependOn: [inspected, adapter, connectivity, representation]`.
- `harness/README.md` contains the inspector endpoint table and the `-inspector-source-timeout` row.
- `.todo` contains the section `## Inspector in the workbench`.
- `git diff --stat` shows no change to `harness/workbench/index.html` or under `harness/inspected/`.
- `progress.md` shows every step `done`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Workbench UI changes. Both panels stay empty.
- Starting the workbench in tests on a fixed port or verifying against a running process (recorded in `.todo`).
- A flag for the inspected URL. The inspector always observes the simulation of the same workbench.
