---
title: "05 - Inspected app, observability and control endpoints"
dependencies: ["02-simulation-actor", "03-prometheus-metrics", "04-health-evaluation"]
effort: "M"
complexity: "medium"
---

# 05 - Inspected app, observability and control endpoints

## Objective

Package `harness/inspected` composes the simulation, metrics and health into an `App` with an explicit `Config`, a lifecycle (`Run`) and one `http.Handler` that serves every route under the path prefix `/inspected`. This step delivers the index, health, metrics and simulation control endpoints and the shared JSON and error conventions. The business API follows in step 06.

## Target Artifacts

| File | Content |
| --- | --- |
| `harness/inspected/doc.go` | Package doc: purpose, path isolation, endpoint table, JSON conventions, error codes, config, lifecycle. |
| `harness/inspected/config.go` | `Config`, `Config.Validate`. |
| `harness/inspected/app.go` | `PathPrefix`, `App`, `New`, `App.Run`, `App.Handler`. |
| `harness/inspected/routes.go` | Route table: pattern, route name, handler. |
| `harness/inspected/observability.go` | Handlers: index, live, ready. Metrics handler construction. |
| `harness/inspected/control.go` | Handlers: simulation status, clock, advance, dependency mode. |
| `harness/inspected/resources.go` | Unexported JSON resource types and converters. |
| `harness/inspected/respond.go` | `writeJSON`, `writeError`, `decodeJSON`, `errorResponse`. |
| `harness/inspected/helpers_test.go` | `testConfig`, `startApp`, `serve`, `decodeBody`. |
| `harness/inspected/config_test.go` | Config tests. |
| `harness/inspected/observability_test.go` | Index, health, metrics tests. |
| `harness/inspected/control_test.go` | Simulation control tests. |
| `.go-arch-lint.yml` | Component `inspected` and rules. |

## Implementation Tasks

1. Write `helpers_test.go`, `config_test.go`, `observability_test.go`, `control_test.go` with every test listed below. Confirm they fail to compile.
2. Create `config.go`.
3. Create `respond.go` and `resources.go`.
4. Create `app.go` with `New`, `Run`, `Handler`.
5. Create `routes.go`, `observability.go`, `control.go`.
6. Create `doc.go`.
7. In `.go-arch-lint.yml` add component `inspected: { in: harness/inspected }` and under `deps:` add `inspected: { mayDependOn: [fulfillment, simulation, metrics, health], canUse: [prometheus] }`.
8. Run `go test ./harness/inspected/...` until it passes, then `task all`.

## Technical Details

### Config (`config.go`)

```go
// Config configures the inspected service. Every field is required and has no default.
type Config struct {
    Seed           uint64        // traffic generator seed
    OrdersPerTick  int           // 0..simulation.MaxOrdersPerTick
    TickInterval   time.Duration // wall-clock time between clock ticks, > 0
    RequestTimeout time.Duration // max wait for a simulation reply per HTTP request, > 0
}

func (c Config) Validate() error
```

`Validate` returns `fmt.Errorf("inspected: tick interval must be positive, got %s", ...)`, the same pattern for `request timeout`, and wraps `simulation.Config{Seed, OrdersPerTick}.Validate()` errors with prefix `inspected: `.

### App (`app.go`)

```go
// PathPrefix isolates every inspected endpoint. The workbench mounts
// Handler at PathPrefix + "/". Handler routes include the prefix.
const PathPrefix = "/inspected"

type App struct {
    cfg      Config
    registry *prometheus.Registry
    metrics  *metrics.Metrics
    sim      *simulation.Simulation
}

func New(cfg Config) (*App, error)
func (a *App) Run(ctx context.Context) error
func (a *App) Handler() http.Handler
```

`New`:

1. `cfg.Validate()`.
2. `registry := prometheus.NewRegistry()`; register `collectors.NewGoCollector()` and `collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})` with `registry.Register`, returning wrapped errors.
3. `metrics.New(registry)`.
4. `simulation.New(simulation.Config{Seed: cfg.Seed, OrdersPerTick: cfg.OrdersPerTick}, fulfillment.StandardCatalog(), m)`.
5. Start no goroutine.

`Run` owns the wall-clock ticker:

```go
func (a *App) Run(ctx context.Context) error {
    ticker := time.NewTicker(a.cfg.TickInterval)
    defer ticker.Stop()
    return a.sim.Run(ctx, ticker.C)
}
```

`Handler` builds a new `http.ServeMux` from the route table. Each handler is wrapped with `a.metrics.InstrumentHandler(routeName, h)`.

### Routes (`routes.go`)

| Pattern | Route name | Handler |
| --- | --- | --- |
| `GET /inspected/{$}` | `index` | `handleIndex` |
| `GET /inspected/health/live` | `health_live` | `handleLive` |
| `GET /inspected/health/ready` | `health_ready` | `handleReady` |
| `GET /inspected/metrics` | `metrics` | `promhttp.HandlerFor(a.registry, promhttp.HandlerOpts{ErrorHandling: promhttp.HTTPErrorOnError})` |
| `GET /inspected/sim` | `sim_status` | `handleSimStatus` |
| `PUT /inspected/sim/clock` | `sim_clock` | `handleSimClock` |
| `POST /inspected/sim/advance` | `sim_advance` | `handleSimAdvance` |
| `PUT /inspected/sim/dependencies/{name}` | `sim_dependency` | `handleSimDependency` |

Patterns are built from `PathPrefix`, not repeated literals. Unknown paths get the `ServeMux` 404; a wrong method gets the `ServeMux` 405.

### Request handling rules

- Every simulation call uses `ctx, cancel := context.WithTimeout(r.Context(), a.cfg.RequestTimeout)`.
- JSON responses use `Content-Type: application/json; charset=utf-8` and `json.NewEncoder(w).Encode`.
- `decodeJSON(w, r, dst)` wraps the body with `http.MaxBytesReader(w, r.Body, 1<<20)`, calls `DisallowUnknownFields`, decodes one value and rejects trailing data (`dec.More()`). Any failure is `400 invalid_request`.
- Error body: `{"error":{"code":"<code>","message":"<err.Error()>"}}`.

`errorResponse(err error) (status int, code string)` in `respond.go`:

| Error | Status | Code |
| --- | --- | --- |
| `simulation.ErrStopped`, `context.DeadlineExceeded`, `context.Canceled` | 503 | `simulation_unavailable` |
| `simulation.ErrInvalidAdvance` | 422 | `invalid_advance` |
| `fulfillment.ErrUnknownDependency` | 404 | `unknown_dependency` |
| `fulfillment.ErrInvalidDependencyMode` | 422 | `invalid_dependency_mode` |
| any other | 500 | `internal_error` |

Matching uses `errors.Is`. Step 06 adds rows to this table.

### JSON resources (`resources.go`)

```go
type indexResource struct {
    Service     string            `json:"service"`     // "inspected"
    Description string            `json:"description"` // "Simulated order fulfillment service"
    Links       map[string]string `json:"links"`
}

type liveResource struct {
    Status string `json:"status"`           // "up" or "down"
    Reason string `json:"reason,omitempty"` // err.Error() when down
}

type healthResource struct {
    Status string          `json:"status"`
    Checks []checkResource `json:"checks"`
}

type checkResource struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Reason string `json:"reason,omitempty"`
}

type simulationResource struct {
    NowTick       uint64               `json:"now_tick"`
    ClockRunning  bool                 `json:"clock_running"`
    Seed          uint64               `json:"seed"`
    OrdersPerTick int                  `json:"orders_per_tick"`
    TickInterval  string               `json:"tick_interval"` // time.Duration.String()
    Dependencies  []dependencyResource `json:"dependencies"`
}

type dependencyResource struct {
    Name string `json:"name"`
    Mode string `json:"mode"`
}

type clockRequest struct {
    Running *bool `json:"running"` // required
}

type advanceRequest struct {
    Ticks int `json:"ticks"`
}

type dependencyModeRequest struct {
    Mode string `json:"mode"`
}

type errorResource struct {
    Error errorBody `json:"error"`
}

type errorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### Handlers

| Handler | Behavior |
| --- | --- |
| `handleIndex` | 200 `indexResource` with links `self` `/inspected/`, `health_live`, `health_ready`, `metrics`, `simulation` (`/inspected/sim`). Step 06 adds `products` and `orders`. |
| `handleLive` | `a.sim.Status(ctx)`. Success: 200 `{"status":"up"}`. Error: 503 `{"status":"down","reason":"..."}`. |
| `handleReady` | `a.sim.Snapshot(ctx)`, `health.Evaluate`. 503 when report status is `down`, else 200. On a simulation error: 503 with status `down` and one check `{"name":"simulation","status":"down","reason":"..."}`. |
| `handleSimStatus` | `a.sim.Status(ctx)`, 200 `simulationResource` (seed, orders per tick, tick interval from `a.cfg`). |
| `handleSimClock` | Decode `clockRequest`; `Running == nil` is `400 invalid_request` with message `running is required`; `a.sim.SetClockRunning`; 200 `simulationResource`. |
| `handleSimAdvance` | Decode `advanceRequest`; `a.sim.Advance(ctx, Ticks)`; 200 `simulationResource`. |
| `handleSimDependency` | Decode `dependencyModeRequest`; `fulfillment.ParseDependencyMode`; `a.sim.SetDependencyMode(ctx, DependencyName(r.PathValue("name")), mode)`; 200 `dependencyResource` of the changed dependency. |

Errors from simulation calls and parsing go through `errorResponse`.

### Test helpers (`helpers_test.go`, package `inspected_test`)

```go
// testConfig returns a config whose clock never fires during a test.
func testConfig() inspected.Config {
    return inspected.Config{Seed: 1, OrdersPerTick: 0, TickInterval: time.Hour, RequestTimeout: time.Second}
}

// startApp creates the app, runs it in a goroutine and on cleanup cancels
// it and requires Run to return nil.
func startApp(t *testing.T, cfg inspected.Config) *inspected.App

// serve runs one request synchronously through h with httptest.NewRecorder.
func serve(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder
```

All handler tests call `app.Handler()` through `serve`. No test opens a network port.

### Tests

`config_test.go`:

| Test | Assertion |
| --- | --- |
| `TestConfigValidate` | Table: valid config passes; `TickInterval` 0, `RequestTimeout` 0, `OrdersPerTick` -1, `OrdersPerTick` 101 each fail; the error message names the field. |
| `TestNewRejectsInvalidConfig` | `inspected.New(inspected.Config{})` returns an error. |

`observability_test.go`:

| Test | Assertion |
| --- | --- |
| `TestIndex` | `GET /inspected/` is 200; every link value starts with `/inspected/`. |
| `TestIndexLinksResolve` | Every link from the index answers `GET` with 200. |
| `TestLive` | `GET /inspected/health/live` is 200 with body `{"status":"up"}`. |
| `TestLiveTimesOutWhenSimulationNotRunning` | `inspected.New` with `RequestTimeout: time.Millisecond`, `Run` not called; `GET /inspected/health/live` is 503 with status `down` and a reason containing `deadline exceeded`. |
| `TestReady` | 200; status `up`; 3 checks named `payment-gateway`, `warehouse`, `inventory`. |
| `TestReadyDownDuringOutage` | `PUT /inspected/sim/dependencies/payment-gateway` `{"mode":"outage"}`; `GET /inspected/health/ready` is 503; status `down`; first check `{"name":"payment-gateway","status":"down","reason":"payment-gateway is in outage: calls fail"}`. |
| `TestReadyDegradedWhenSlow` | Mode `slow`; ready is 200 with status `degraded`. |
| `TestMetrics` | `GET /inspected/metrics` is 200; `Content-Type` starts with `text/plain`; body has a line `fulfillment_simulation_tick 0` and a line starting with `go_goroutines`. |
| `TestMetricsFollowAdvance` | `POST /inspected/sim/advance` `{"ticks":3}`; metrics body has line `fulfillment_simulation_tick 3`. |
| `TestHTTPRequestsAreInstrumented` | `GET /inspected/health/live`, then metrics body has line `inspected_http_requests_total{code="200",handler="health_live",method="get"} 1`. |
| `TestUnknownPathAndMethod` | `GET /inspected/nope` is 404; `POST /inspected/health/live` is 405. |

`control_test.go`:

| Test | Assertion |
| --- | --- |
| `TestSimStatus` | `GET /inspected/sim` is 200 with `now_tick` 0, `clock_running` true, `seed` 1, `orders_per_tick` 0, `tick_interval` `1h0m0s`, dependencies `[{payment-gateway, healthy}, {warehouse, healthy}]`. |
| `TestSimClock` | `PUT /inspected/sim/clock` `{"running":false}` is 200 with `clock_running` false; body `{}` is 400 `invalid_request`. |
| `TestSimAdvance` | `{"ticks":5}` is 200 with `now_tick` 5; `{"ticks":0}` and `{"ticks":1001}` are 422 `invalid_advance`; body `{` is 400; `{"ticks":1,"x":1}` is 400. |
| `TestSimDependency` | `PUT .../warehouse` `{"mode":"slow"}` is 200 `{"name":"warehouse","mode":"slow"}`; `.../unknown` is 404 `unknown_dependency`; `{"mode":"down"}` is 422 `invalid_dependency_mode`. |

## Verification

```powershell
go test ./harness/inspected/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/inspected/...` exits 0 and runs every test listed above.
- `grep -rn "\"/inspected" harness/inspected --include=*.go` prints, outside `*_test.go` files, exactly one match: the `PathPrefix` declaration. All routes and links are built from `PathPrefix`.
- `harness/inspected` does not import `harness/workbench`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Products and orders endpoints (step 06).
- Mounting in the workbench and command-line flags (step 07).
- Authentication of the control API.
- Graceful drain of in-flight simulation requests beyond context cancellation.
