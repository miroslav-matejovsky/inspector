---
title: "03 - Prometheus metrics"
dependencies: ["01-fulfillment-domain"]
effort: "S"
complexity: "low"
---

# 03 - Prometheus metrics

## Objective

Package `harness/inspected/metrics` defines every Prometheus metric of the inspected service on a caller-supplied registerer. It turns `fulfillment` events into counters and histograms, turns a `fulfillment.Snapshot` into gauges, and instruments HTTP handlers. It satisfies the `simulation.Recorder` interface without importing `simulation`. The metric set passes `promlint`.

## Target Artifacts

| File | Content |
| --- | --- |
| `go.mod`, `go.sum` | `github.com/prometheus/client_golang v1.24.1` and its transitive modules. |
| `harness/inspected/metrics/doc.go` | Package doc with the full metric table below. |
| `harness/inspected/metrics/metrics.go` | `Metrics`, `New`, `Record`, `Observe`, `InstrumentHandler`. |
| `harness/inspected/metrics/metrics_test.go` | Tests. |
| `.go-arch-lint.yml` | Vendor `prometheus`, component `metrics`, rules. |

## Implementation Tasks

1. Run `go get github.com/prometheus/client_golang@v1.24.1`.
2. Write `metrics_test.go` with every test listed below. Confirm it fails to compile.
3. Create `metrics.go`.
4. Create `doc.go`.
5. In `.go-arch-lint.yml` add:
   - top-level `vendors:` section with `prometheus: { in: "github.com/prometheus/**" }`;
   - component `metrics: { in: harness/inspected/metrics }`;
   - under `deps:` the entry `metrics: { mayDependOn: [fulfillment], canUse: [prometheus] }`.
6. Run `go test ./harness/inspected/metrics/...` until it passes, then `task all`.

## Technical Details

### API

```go
// Metrics owns the metric collectors of the inspected service.
// Record and Observe are called by the simulation actor only.
// InstrumentHandler wrappers are safe for concurrent HTTP use
// (prometheus collectors are internally synchronized).
type Metrics struct { /* one field per collector below */ }

func New(reg prometheus.Registerer) (*Metrics, error)
func (m *Metrics) Record(events []fulfillment.Event)
func (m *Metrics) Observe(s fulfillment.Snapshot)
func (m *Metrics) InstrumentHandler(name string, h http.Handler) http.Handler
```

`New` creates all collectors, registers each with `reg.Register` and returns the first error wrapped as `fmt.Errorf("metrics: register %s: %w", name, err)`. It never uses the global default registry.

### Metric catalog

| Name | Type | Labels | HELP text | Updated by |
| --- | --- | --- | --- | --- |
| `fulfillment_orders_placed_total` | counter | `channel` | `Orders accepted by the fulfillment service, by placement channel.` | `Record`: `OrderPlaced` |
| `fulfillment_order_transitions_total` | counter | `from`, `to`, `reason` | `Order status transitions, by source status, target status and reason.` | `Record`: `OrderStatusChanged` |
| `fulfillment_order_duration_ticks` | histogram, buckets `1, 2, 3, 4, 6, 8, 12` | `status` | `Simulation ticks from order placement to a terminal status.` | `Record`: `OrderStatusChanged` with terminal `To`, value `At - PlacedAt` |
| `fulfillment_dependency_calls_total` | counter | `dependency`, `result` | `Calls to downstream dependencies, by dependency and result.` | `Record`: `DependencyCalled`, `result` is `success` or `failure` |
| `fulfillment_product_restocks_total` | counter | `sku` | `Warehouse restocks, by product SKU.` | `Record`: `ProductRestocked` |
| `fulfillment_orders_in_progress` | gauge | `status` | `Orders not yet in a terminal status, by status.` | `Observe`: count of `pending` and `paid` orders |
| `fulfillment_product_stock` | gauge | `sku` | `Units in stock, by product SKU.` | `Observe`: `Product.Stock` |
| `fulfillment_dependency_up` | gauge | `dependency` | `Whether a downstream dependency accepts calls (1) or is in outage (0).` | `Observe`: 0 for `outage`, else 1 |
| `fulfillment_dependency_mode` | gauge | `dependency`, `mode` | `Current simulated mode of a downstream dependency, 1 for the active mode and 0 otherwise.` | `Observe`: one series per `DependencyModes()` value |
| `fulfillment_simulation_tick` | gauge | none | `Current simulation tick.` | `Observe`: `Snapshot.Now` |
| `inspected_http_requests_total` | counter | `handler`, `code`, `method` | `HTTP requests handled by the inspected service, by handler, status code and method.` | `InstrumentHandler` via `promhttp.InstrumentHandlerCounter` |
| `inspected_http_request_duration_seconds` | histogram, buckets `prometheus.DefBuckets` | `handler`, `method` | `HTTP request latency of the inspected service, by handler and method.` | `InstrumentHandler` via `promhttp.InstrumentHandlerDuration` |

### Series initialization in `New`

So that every known series is visible from the first scrape, `New` creates these series with value 0:

- `fulfillment_orders_placed_total` for each `fulfillment.Channels()` value.
- `fulfillment_order_transitions_total` for each `fulfillment.AllowedTransitions()` rule.
- `fulfillment_dependency_calls_total` for each `fulfillment.DependencyNames()` value times `success`, `failure`.
- `fulfillment_order_duration_ticks` for `shipped` and `failed`.
- `fulfillment_orders_in_progress` for `pending` and `paid`.

`fulfillment_product_restocks_total` series appear on the first restock of a SKU. Gauges that depend on the catalog or dependencies are set by the first `Observe`, which the actor calls when `Run` starts.

### `Record`

Type switch over each event. Unknown event type: `panic(fmt.Sprintf("metrics: unhandled event %T", e))`. The event set is closed in `fulfillment`, so this is a programming error.

### `InstrumentHandler`

```go
func (m *Metrics) InstrumentHandler(name string, h http.Handler) http.Handler {
    labels := prometheus.Labels{"handler": name}
    return promhttp.InstrumentHandlerDuration(m.httpDuration.MustCurryWith(labels),
        promhttp.InstrumentHandlerCounter(m.httpRequests.MustCurryWith(labels), h))
}
```

`promhttp` reports the `method` label in lower case (`get`, `post`, `put`).

### Tests (`metrics_test.go`, package `metrics_test`)

Every test creates its own `prometheus.NewRegistry()` and uses `github.com/prometheus/client_golang/prometheus/testutil`.

| Test | Setup | Assertion |
| --- | --- | --- |
| `TestNewInitializesKnownSeries` | `New(reg)` | `testutil.GatherAndCompare` for `fulfillment_orders_placed_total` equals HELP, TYPE and the two series `channel="api"` and `channel="simulation"` with value 0; `testutil.GatherAndCount(reg, "fulfillment_order_transitions_total") == 6`; `GatherAndCount(reg, "fulfillment_dependency_calls_total") == 4`. |
| `TestNewFailsOnDuplicateRegistration` | `New(reg)` twice on one registry | Second call returns a non-nil error. |
| `TestRecordEvents` | `Record` of `OrderPlaced{api}`, `DependencyCalled{payment-gateway, true}`, `OrderStatusChanged{pending->paid, payment_authorized}`, `DependencyCalled{warehouse, false}`, `ProductRestocked{sku-003}` | Via `GatherAndCompare`: placed `api` 1, `simulation` 0; transition `pending/paid/payment_authorized` 1, the other 5 series 0; calls `payment-gateway/success` 1, `warehouse/failure` 1, others 0; restocks `sku-003` 1. |
| `TestRecordTerminalDuration` | `Record` `OrderStatusChanged{paid->shipped, PlacedAt 0, At 2}` | `fulfillment_order_duration_ticks` for `status="shipped"` has `_count` 1, `_sum` 2, bucket `le="2"` 1, bucket `le="1"` 0. |
| `TestObserveSnapshot` | `fulfillment.NewState(StandardCatalog())`, `payment-gateway` to `outage`, place `sku-001` x1, `Observe(state.Snapshot())` | `fulfillment_simulation_tick` 0; `in_progress` `pending` 1, `paid` 0; `product_stock` `sku-001` 40 (5 series); `dependency_up` `payment-gateway` 0, `warehouse` 1; `dependency_mode` `payment-gateway/outage` 1, `payment-gateway/healthy` 0, `payment-gateway/slow` 0 (6 series). |
| `TestInstrumentHandler` | Wrap a handler that writes status 418 as `probe`, serve `GET /` with `httptest.NewRecorder` | `inspected_http_requests_total{code="418",handler="probe",method="get"}` is 1; `GatherAndCount(reg, "inspected_http_request_duration_seconds") == 1`. |
| `TestMetricsPassLint` | `New`, one `Observe`, one `Record` of every event type, one instrumented request | `testutil.GatherAndLint(reg)` returns zero problems. |

## Verification

```powershell
go get github.com/prometheus/client_golang@v1.24.1
go test ./harness/inspected/metrics/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go.mod` requires `github.com/prometheus/client_golang v1.24.1`.
- `go test ./harness/inspected/metrics/...` exits 0 and runs every test listed above.
- `TestMetricsPassLint` passes (zero `promlint` problems).
- `git grep -n -e "prometheus.MustRegister" -e "prometheus.DefaultRegisterer" -e "promauto" -- harness` prints nothing.
- `harness/inspected/metrics` imports no project package other than `harness/inspected/fulfillment`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Serving `/inspected/metrics` (step 05).
- Go runtime and process collectors (registered by `inspected.New` in step 05).
- OpenMetrics format, exemplars, native histograms.
- Metrics for the simulation control API beyond the generic HTTP instrumentation.
