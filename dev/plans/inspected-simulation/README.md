# Inspected simulation

## Goal

Build `harness/inspected`: a simulated order fulfillment service. The workbench starts it in-process and serves it under the URL path `/inspected`. It exposes a JSON business API, health checks and Prometheus metrics that faithfully describe its own state. A future Inspector gets a realistic, reproducible system to observe, explain and navigate.

## Scope

In scope:

- Deterministic, tick-driven fulfillment domain: products, orders, two downstream dependencies (payment gateway, warehouse).
- One simulation actor that owns all mutable state, generates seeded traffic and consumes a clock.
- Prometheus metrics on a dedicated registry, served at `/inspected/metrics`.
- Liveness and readiness health checks with a reason for every non-healthy check.
- JSON API for products and orders.
- Simulation control API under `/inspected/sim`: pause or resume the clock, advance N ticks, set a dependency mode.
- Workbench wiring: flags, startup, shutdown, supervision, `task workbench`.
- Package documentation (`doc.go`), `harness/README.md`, root `README.md`, `.todo` live verification entry.

Out of scope:

- Any Inspector code. The `observation`, `explanation`, `navigation`, `representation` and `connectivity` folders stay untouched.
- Workbench UI changes. Both panels stay empty.
- Persistence. State lives in memory. A restart rebuilds state from the seed.
- Authentication, TLS, pagination, rate limiting, CORS.

## Simulated business

The inspected service sells a fixed catalog of 5 products. Orders move through a payment stage and a shipping stage. Each stage calls one downstream dependency.

| Concept | Identity | Meaning |
| --- | --- | --- |
| Product | SKU, `sku-001` .. `sku-005` | Sellable item with price, stock, capacity and reorder point. |
| Order | `ord-000001`, sequential | Request for `quantity` units of one SKU. Carries its full transition history with reasons. |
| Dependency | `payment-gateway`, `warehouse` | Downstream system. Mode is `healthy`, `slow` or `outage`. |
| Tick | unsigned integer, starts at 0 | Logical time. All durations are counted in ticks. |

Order lifecycle:

```text
            payment_authorized            shipped
 pending  ----------------------> paid ------------> shipped
    |                               |
    | payment_declined              | out_of_stock
    | payment_gateway_unavailable   | warehouse_unavailable
    v                               v
  failed                          failed
```

Every observable outcome has a recorded cause: a transition reason on the order, a reason on the health check, a label on the metric.

## Architecture impact

New packages (all under `harness/inspected`):

| Package | Responsibility | May depend on |
| --- | --- | --- |
| `harness/inspected/fulfillment` | Pure business rules. No goroutines, no I/O, no wall clock, no randomness. | stdlib |
| `harness/inspected/simulation` | Actor that owns the `fulfillment.State`, seeded traffic generator, clock handling. | `fulfillment` |
| `harness/inspected/metrics` | Prometheus metric definitions, event and snapshot recording, HTTP instrumentation. | `fulfillment`, prometheus |
| `harness/inspected/health` | Pure health evaluation of a snapshot. | `fulfillment` |
| `harness/inspected` | Composition: config, app lifecycle, HTTP routes, JSON contracts. | all of the above, prometheus |

Changed:

| Artifact | Change |
| --- | --- |
| `harness/workbench` | New `Config`, `ParseConfig`; `Handler` mounts the inspected handler under `/inspected/`; `Run` starts and supervises the inspected app. |
| `cmd/workbench/main.go` | Uses `workbench.ParseConfig`. |
| `Taskfile.yml` | `task workbench` passes all required flags. |
| `.go-arch-lint.yml` | New components, `prometheus` vendor, dependency rules. |
| `go.mod`, `go.sum` | Adds `github.com/prometheus/client_golang v1.24.1` and its transitive modules. |

Dependency graph:

```text
cmd/workbench -> harness/workbench -> harness/inspected -+-> simulation -> fulfillment
                                                         +-> metrics ----> fulfillment
                                                         +-> health -----> fulfillment
```

Endpoints (all served by the workbench HTTP server, all under `/inspected`):

| Method | Path | Purpose | Status codes | Step |
| --- | --- | --- | --- | --- |
| GET | `/inspected/` | Index of links to every endpoint | 200 | 05, 06 |
| GET | `/inspected/health/live` | Liveness: simulation actor answers | 200, 503 | 05 |
| GET | `/inspected/health/ready` | Readiness with per-check status and reason | 200, 503 | 05 |
| GET | `/inspected/metrics` | Prometheus text exposition | 200 | 05 |
| GET | `/inspected/sim` | Simulation status | 200, 503 | 05 |
| PUT | `/inspected/sim/clock` | Pause or resume the clock | 200, 400, 503 | 05 |
| POST | `/inspected/sim/advance` | Advance N ticks | 200, 400, 422, 503 | 05 |
| PUT | `/inspected/sim/dependencies/{name}` | Set dependency mode | 200, 400, 404, 422, 503 | 05 |
| GET | `/inspected/api/products` | List products | 200, 503 | 06 |
| GET | `/inspected/api/products/{sku}` | One product | 200, 404, 503 | 06 |
| GET | `/inspected/api/orders` | List retained orders, filters `status`, `sku` | 200, 400, 503 | 06 |
| GET | `/inspected/api/orders/{id}` | One order with history | 200, 404, 503 | 06 |
| POST | `/inspected/api/orders` | Place an order | 201, 400, 422, 503 | 06 |

Workbench flags (all required, no defaults):

| Flag | Type | Meaning | Validation |
| --- | --- | --- | --- |
| `-addr` | string | Listen address | not empty |
| `-inspected-seed` | uint64 | Seed of the traffic generator | any value |
| `-inspected-orders-per-tick` | int | Simulated orders placed per tick | 0..100 |
| `-inspected-tick-interval` | duration | Wall-clock time between clock ticks | > 0 |
| `-inspected-request-timeout` | duration | Max wait for a simulation reply per HTTP request | > 0 |

## Principles evaluation

| Principle | How this plan supports it |
| --- | --- |
| [Observe](../../principles/2-observe.md) | State is exposed through three read-only channels a future Inspector can consume: GET API, metrics, health. The control API lives under a separate path (`/inspected/sim`) and is for the harness operator only, so observation stays read-only and never needs to influence the inspected system. Metric gauges and API responses come from the same actor-owned state, so they agree. |
| [Explain](../../principles/1-explain.md) | Every outcome carries its cause: order history holds each transition with tick and reason; failed orders carry `failure_reason`; health checks carry a reason naming the dependency and its mode; metrics carry `reason` and `dependency` labels. Seeded traffic plus logical ticks make every scenario reproducible, so an explanation can be checked against a known, injected cause. |
| [Navigate](../../principles/3-navigate.md) | Concepts share stable identifiers across all channels: SKU links order to product, dependency name is the health check name and the metric `dependency` label, transition reasons name the failing dependency. JSON resources carry `links` (order to product, product to its orders) and `/inspected/` links every endpoint. |
| [Domain Map](../../principles/4-domain-map.md) | The simulation is an inspected system, the external source that Connectivity will integrate with later. No Inspector domain is implemented here. |

## Deliverables

- Packages `harness/inspected`, `harness/inspected/fulfillment`, `harness/inspected/simulation`, `harness/inspected/metrics`, `harness/inspected/health`, each with `doc.go` and tests.
- Updated `harness/workbench` package, `cmd/workbench/main.go`, `Taskfile.yml`, `.go-arch-lint.yml`, `go.mod`, `go.sum`.
- Updated `harness/README.md`, root `README.md`, `.todo`.

## Success criteria

- `task all` exits with code 0.
- Every test named in steps 01 to 07 exists and passes.
- `go list ./harness/inspected/...` lists exactly 5 packages.
- `git diff --stat` shows no change under `observation/`, `explanation/`, `navigation/`, `representation/`, `connectivity/`, `harness/workbench/index.html`.
- Test `TestWorkbenchServesInspected` (step 07) gets HTTP 200 from `/inspected/health/live` through `workbench.Handler`.
- `.todo` contains the live verification section `Inspected simulation` (step 07).

## Steps

| Step | Title | Depends on |
| --- | --- | --- |
| [01](01-fulfillment-domain.md) | Fulfillment domain model | - |
| [02](02-simulation-actor.md) | Simulation actor | 01 |
| [03](03-prometheus-metrics.md) | Prometheus metrics | 01 |
| [04](04-health-evaluation.md) | Health evaluation | 01 |
| [05](05-inspected-app.md) | Inspected app, observability and control endpoints | 02, 03, 04 |
| [06](06-business-api.md) | Business API | 05 |
| [07](07-workbench-integration.md) | Workbench integration and documentation | 06 |

Supporting documents: [alternatives.md](alternatives.md), [assessment.md](assessment.md), [progress.md](progress.md).
