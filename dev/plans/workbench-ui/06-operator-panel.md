---
title: "06 - Operator panel"
dependencies: ["04-dashboard-views"]
effort: "M"
complexity: "medium"
---

# 06 - Operator panel

## Objective

The workbench serves an operator page under `/operator/` that shows the simulation status, the dependencies with their modes and the products, and lets the harness operator pause or resume the clock, advance ticks, set a dependency mode and place an order. The operator is harness tooling: it calls the inspected HTTP API with its own client, like the `curl` commands in `harness/README.md`. It is not part of the Inspector library and does not use `connectivity`, which stays GET-only.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/operator.go` | New: `OperatorPathPrefix`, `OperatorHandler`, inspected API client, page and form handlers. |
| `harness/workbench/operator.html` | New: templates `operator` and `operator-error`, embedded. |
| `harness/workbench/operator_test.go` | New tests, package `workbench_test`. |
| `harness/workbench/workbench.go` | `Handler(inspectedHandler, inspectorHandler, operatorHandler)`; `Run` builds the operator handler. |
| `harness/workbench/config.go` | `Config.OperatorTimeout`; flag `-operator-timeout`; validation. |
| `harness/workbench/config_test.go` | New flag. |
| `harness/workbench/workbench_test.go`, `scenario_test.go` | `Handler` calls with three handlers; `validConfig(t)` sets `OperatorTimeout`; `TestHandlerMountsOperator`. |
| `harness/workbench/doc.go` | Section `# Operator`; configuration list. |
| `cmd/workbench/main.go` | Start message names the operator URL. |
| `Taskfile.yml` | `workbench` passes `-operator-timeout 2s`. |
| `harness/README.md` | Flag table row; operator section with endpoint table. |

## Implementation Tasks

1. Write `operator_test.go` and the config test changes. Confirm they fail.
2. Create `operator.html`, then `operator.go`.
3. Add `OperatorTimeout` and the flag; change `Handler` and `Run`; update the existing tests for the new `Handler` signature.
4. Update `cmd/workbench/main.go`, `Taskfile.yml`, `doc.go`, `harness/README.md`.
5. `go test ./harness/workbench/...`, then `task all`.

## Technical Details

### API (`operator.go`)

```go
// OperatorPathPrefix isolates the operator panel. Handler mounts the operator
// handler at OperatorPathPrefix + "/"; its routes include the prefix.
const OperatorPathPrefix = "/operator"

// OperatorHandler serves the operator panel of the inspected service at
// inspectedURL, for example "http://127.0.0.1:8080/inspected". The panel reads
// and changes the simulation through the inspected HTTP API; each request to
// the service is bounded by timeout. inspectedURL is an absolute http or https
// URL without trailing slash; timeout must be positive.
func OperatorHandler(inspectedURL string, timeout time.Duration) (http.Handler, error)
```

The returned handler is `http.NewCrossOriginProtection().Handler(mux)`.

### Routes

| Route | Form fields | Inspected request | Success |
| --- | --- | --- | --- |
| `GET /operator/{$}` | none | `GET /sim`, `GET /api/products` | 200 page |
| `POST /operator/clock` | `running` (`strconv.ParseBool`) | `PUT /sim/clock` `{"running": <bool>}` | 303 to `/operator/` |
| `POST /operator/advance` | `ticks` (`strconv.Atoi`) | `POST /sim/advance` `{"ticks": <n>}` | 303 to `/operator/` |
| `POST /operator/dependencies/{name}` | `mode` | `PUT /sim/dependencies/<url.PathEscape(name)>` `{"mode": "<mode>"}` | 303 to `/operator/` |
| `POST /operator/orders` | `sku`, `quantity` (`strconv.Atoi`) | `POST /api/orders` `{"sku": "<sku>", "quantity": <n>}` | 303 to `/operator/` |

Paths are relative to `inspectedURL`. Forms are parsed with a 64 KiB body limit.

Errors, rendered by template `operator-error` with a link back to `/operator/`:

| Condition | Status | Code |
| --- | --- | --- |
| form too large, unparsable, or a number or boolean field does not parse | 400 | `invalid_form` |
| inspected answers a non-2xx status on a POST route | the same status | the inspected error code, message included |
| request to the inspected service fails (connection, timeout, undecodable body), or the page read gets a non-2xx status | 502 | `inspected_unavailable` |

The operator does not validate modes, SKUs or tick counts; the inspected service does, and its answer is shown.

### Client

```go
// operator calls the inspected API. It is harness tooling and may send any
// method, unlike connectivity.HTTPReader.
type operator struct {
    base    string // inspected URL, without trailing slash
    timeout time.Duration
    client  *http.Client
}

// call sends method and path with body encoded as JSON when body is not nil,
// and decodes a 2xx JSON response into dst when dst is not nil. A non-2xx
// response is returned as *inspectedError.
func (o *operator) call(ctx context.Context, method, path string, body, dst any) error

// inspectedError is an error answer of the inspected service.
type inspectedError struct {
    Status  int
    Code    string // from {"error":{"code","message"}}; empty when the body has none
    Message string
}
```

Own copies of the inspected resources, limited to the fields the page shows (unknown fields ignored):

```go
type simulationDoc struct {
    NowTick       uint64          `json:"now_tick"`
    ClockRunning  bool            `json:"clock_running"`
    Seed          uint64          `json:"seed"`
    OrdersPerTick int             `json:"orders_per_tick"`
    TickInterval  string          `json:"tick_interval"`
    Dependencies  []dependencyDoc `json:"dependencies"`
}

type dependencyDoc struct {
    Name string `json:"name"`
    Mode string `json:"mode"`
}

type productListDoc struct {
    Products []productDoc `json:"products"`
}

type productDoc struct {
    SKU   string `json:"sku"`
    Name  string `json:"name"`
    Stock int    `json:"stock"`
}
```

### Page data and template (`operator.html`)

```go
// dependencyModes are the modes the dependency form offers. They are the modes
// of the inspected simulation; TestOperatorOffersEveryDependencyMode keeps
// them in sync.
var dependencyModes = []string{"healthy", "slow", "outage"}

type operatorPage struct {
    Simulation   simulationDoc
    Dependencies []dependencyRow
    Products     []productDoc
    Modes        []string
    Links        []string // raw inspected endpoints, site-relative
}

type dependencyRow struct {
    Name, Mode string
    Action     string // OperatorPathPrefix + "/dependencies/" + url.PathEscape(Name)
}
```

`Links`: `inspected.PathPrefix` plus `/`, `/health/ready`, `/metrics`, `/sim`, `/api/dependencies`, `/api/products`, `/api/orders`.

Template `operator` (standalone document, same color tokens and dark mode as `index.html`; fields only, no methods):

- `Simulation`: `tick {{.Simulation.NowTick}}`, `clock running` or `clock paused`, seed, orders per tick, tick interval, and a `reload` link to `/operator/`.
- Clock form: hidden `running` set to the opposite of `ClockRunning`, button `Pause clock` or `Resume clock`.
- Advance form: `ticks` (`type="number" min="1" value="1"`), button `Advance`.
- Dependencies table: name, mode, form to `{{.Action}}` with `<select name="mode">` rendering `<option value="{{.}}">` for each of `$.Modes`, the current mode `selected`, button `Set`.
- Place order form: `<select name="sku">` over `Products` (SKU, name, stock), `quantity` (`type="number" min="1" value="1"`), button `Place order`.
- Raw endpoints: one link per entry of `Links`, `target="_blank"`.

Template `operator-error`: status, code, message and `<a href="/operator/">Back</a>`.

### Configuration

```go
OperatorTimeout time.Duration // max wait for one request of the operator panel to the inspected service, must be positive
```

Flag: `fs.DurationVar(&cfg.OperatorTimeout, "operator-timeout", 0, "max wait for one request of the operator panel to the inspected service, for example 2s (required)")`. `Validate`: `workbench: operator timeout must be positive, got %s`.

### Workbench wiring (`workbench.go`)

```go
// Handler serves the workbench page at "/" and delegates, without stripping
// prefixes, inspected.PathPrefix+"/" to inspectedHandler,
// InspectorPathPrefix+"/" to inspectorHandler and OperatorPathPrefix+"/" to
// operatorHandler.
func Handler(inspectedHandler, inspectorHandler, operatorHandler http.Handler) http.Handler
```

`Run` builds `OperatorHandler("http://"+ln.Addr().String()+inspected.PathPrefix, cfg.OperatorTimeout)` after the inspector handler, closing the listener on error like the inspector branch does.

`cmd/workbench/main.go` start message: `workbench listening on http://%s (inspected at http://%s%s/, inspector at http://%s%s/, operator at http://%s%s/)`.

### Tests

`harness/workbench/operator_test.go`. Helper `operatorScenario(t) (http.Handler, *httptest.Server)`: `startInspected(t)` (its clock never fires), `httptest.NewServer(app.Handler())`, `OperatorHandler(srv.URL+inspected.PathPrefix, time.Second)`. State is checked with GET requests to the test server.

| Test | Assertion |
| --- | --- |
| `TestOperatorPage` | `GET /operator/`: 200, `text/html`; body contains `tick 0`, `clock running`, `Pause clock`, `payment-gateway`, `warehouse`, `sku-001`, `href="/inspected/metrics"`. |
| `TestOperatorOffersEveryDependencyMode` | For each mode of `fulfillment.DependencyModes()` the page contains `<option value="<mode>"`. Test files may import `fulfillment`; arch-lint excludes tests. |
| `TestOperatorSetsDependencyMode` | `POST /operator/dependencies/payment-gateway` `mode=outage`: 303, `Location` `/operator/`; `GET /inspected/api/dependencies/payment-gateway` has mode `outage`. |
| `TestOperatorPausesAndResumesClock` | `running=false`: `GET /inspected/sim` has `clock_running` false; `running=true`: true. |
| `TestOperatorAdvances` | `ticks=3`: `now_tick` is 3. |
| `TestOperatorPlacesOrder` | `sku=sku-001&quantity=2`: 303; `GET /inspected/api/orders` has one order with SKU `sku-001` and quantity 2. |
| `TestOperatorRejectsInvalidForm` | `running=maybe`, `ticks=x`, `quantity=x` (with `sku=sku-001`): each 400 with `invalid_form`; `now_tick` stays 0 and no order exists. |
| `TestOperatorPassesInspectedErrors` | `mode=broken`: 422 with `invalid_dependency_mode`. `/operator/dependencies/nope` `mode=slow`: 404 `unknown_dependency`. `ticks=0`: 422 `invalid_advance`. `sku=nope&quantity=1`: 422 `unknown_product`. |
| `TestOperatorInspectedUnavailable` | Operator for a closed `httptest` server, timeout 200 ms: `GET /operator/` is 502 with `inspected_unavailable`. |
| `TestOperatorRejectsCrossSite` | `POST /operator/advance` `ticks=1` with `Sec-Fetch-Site: cross-site`: 403; `now_tick` stays 0. |
| `TestOperatorHandlerRejectsInvalidConfig` | URLs `""`, `ftp://h/inspected`, `http://h/inspected/` and timeout 0 each give an error. |

`harness/workbench/workbench_test.go`: every `Handler` call gets a third handler; new `TestHandlerMountsOperator`: `GET /operator/` reaches only the operator stub. `validConfig(t)` sets `OperatorTimeout: time.Second`.

`harness/workbench/config_test.go`: `allFlags` gains `{"operator-timeout", "2s"}`; `TestParseConfig` expects `OperatorTimeout: 2 * time.Second`; the missing-flags message ends with `-inspector-dashboards-file, -inspector-source-timeout, -operator-timeout`; `TestParseConfigRejectsInvalidValues` gains `zero operator timeout`, `negative operator timeout`, `unparsable operator timeout`.

## Verification

```powershell
go test ./harness/workbench/...
go-arch-lint check
task deadcode
task all
```

## Acceptance Criteria

- `go test ./harness/workbench/...` exits 0 and runs every test listed above.
- `harness/workbench/operator.go` does not import `github.com/miroslav-matejovsky/inspector/connectivity`.
- `Taskfile.yml` task `workbench` contains `-operator-timeout 2s`.
- `harness/README.md` flag table lists `-operator-timeout`.
- `task all` exits 0.

## Non-Goals

- Embedding the operator page in the workbench panel (step 07).
- Showing orders on the operator page; orders are observed through the inspector.
- Changes to `harness/inspected`.
