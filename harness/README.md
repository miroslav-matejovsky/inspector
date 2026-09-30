# Harness

Development tooling for running Inspector against something to inspect.

## workbench

`workbench/` is a Go package that serves a web page with one **workbench** panel. It mounts the inspected simulation under `/inspected/` on the same server and runs a `source.Source` that reads the simulation through the workbench listener.

The entry point is `cmd/workbench`. Start it with `task workbench` and open http://localhost:8080. Every setting is a required flag without a default:

| Flag | Meaning |
| --- | --- |
| `-addr` | Listen address, for example `localhost:8080`. |
| `-log-dir` | Directory of the log files, created when missing, for example `logs`. |
| `-log-level` | Lowest level written to the log file: `debug`, `info`, `warn` or `error`. |
| `-inspected-seed` | Seed of the traffic generator. The same seed produces the same traffic. |
| `-inspected-orders-per-tick` | Simulated orders placed per tick, 0 to 100. |
| `-inspected-tick-interval` | Wall-clock time between simulation ticks, for example `1s`. |
| `-inspected-request-timeout` | Max wait for the simulation per HTTP request, for example `2s`. |
| `-source-database` | SQLite file of the collected signals, for example `data/source.db`. Its directory is created when missing. |
| `-source-interval` | Time between collection rounds, for example `5s`. |
| `-source-timeout` | Max wait for one read of one target, for example `2s`. |
| `-source-retention` | Signals older than this are deleted after each round, for example `10m`. |
| `-source-max-body-bytes` | A larger body is stored as a failed read, for example `2097152`. |
| `-source-target` | `name=path` of a workbench path to read, repeated, for example `health_ready=/inspected/health/ready`. |

Each run writes its log to `logs/workbench-<yyyyMMdd-HHmmss>.log`; the path is printed at startup. Collected signals are kept in `data/source.db`.

Page. The page shows the raw view of package `view`: a full-width table with one row per target (stored signals, then time, status, duration, size, content type and error of the latest signal), and below it the latest signal of each target with its body, in two columns. On screens up to 1100px wide the signals use one column. Every 2 seconds the panel is refreshed in place, without a page reload, and scroll positions are kept. JSON bodies are indented and highlighted, metrics are highlighted, other bodies are plain text; at most 500 lines per body are shown. `task workbench` reads the index, both health endpoints, the metrics and the three API collections. It does not read `/inspected/sim`.

Controls. The page header has buttons `healthy`, `slow` and `outage` for `payment-gateway` and `warehouse`. The button of the current mode, read from `GET /inspected/sim`, is filled in the color of its mode. A button sets the dependency mode through `PUT /inspected/sim/dependencies/{name}` and refreshes the panel.

## inspected

`inspected/` is a simulated order fulfillment service. It is the system that Inspector will inspect. It exposes a JSON API, health checks and Prometheus metrics, all under the path `/inspected`.

Simulated domain:

- **Products**: 5 products (`sku-001` .. `sku-005`) with stock. A restock refills a product every 10 ticks when stock is at or below its reorder point.
- **Orders**: an order goes `pending` -> `paid` -> `shipped`, or ends `failed`. Every transition is stored with its tick and reason.
- **Dependencies**: `payment-gateway` (used while pending) and `warehouse` (used while paid). Each is `healthy`, `slow` or `outage`.
- **Ticks**: time is a logical tick. The workbench advances it on a timer, and the control API can advance it by hand.

Packages:

| Package | Responsibility |
| --- | --- |
| `inspected/fulfillment` | Business rules as a pure state machine. |
| `inspected/simulation` | Actor that owns the state, generates seeded traffic and consumes the clock. |
| `inspected/metrics` | Prometheus metrics and HTTP instrumentation. |
| `inspected/health` | Readiness evaluation with a reason for every check that is not up. |
| `inspected` | Configuration, lifecycle and HTTP routes. |

Endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/inspected/` | Index of links to every endpoint |
| GET | `/inspected/health/live` | Liveness: the simulation answers |
| GET | `/inspected/health/ready` | Readiness with per-check status and reason, 503 when down |
| GET | `/inspected/metrics` | Prometheus text exposition |
| GET | `/inspected/sim` | Simulation status |
| PUT | `/inspected/sim/clock` | Pause or resume the clock, body `{"running":false}` |
| POST | `/inspected/sim/advance` | Advance N ticks, body `{"ticks":3}` |
| PUT | `/inspected/sim/dependencies/{name}` | Set a dependency mode, body `{"mode":"outage"}` |
| GET | `/inspected/api/dependencies` | List dependencies with their mode |
| GET | `/inspected/api/dependencies/{name}` | One dependency |
| GET | `/inspected/api/products` | List products |
| GET | `/inspected/api/products/{sku}` | One product |
| GET | `/inspected/api/orders` | List orders, filters `status` and `sku` |
| GET | `/inspected/api/orders/{id}` | One order with its history |
| POST | `/inspected/api/orders` | Place an order, body `{"sku":"sku-001","quantity":2}` |

The `/inspected/sim` endpoints control the simulation for the harness operator. Everything else is read-only observation, except placing an order.

Causes. The service states the causes it knows as links. Every order history entry links the dependency or product that decided it (`cause`). An open order links the dependency it waits on (`links.waiting_on`). Every readiness check links the resources that determine its status (`causes`): its dependency, or the products with zero stock for the inventory check.

### Manual scenarios

Pause the clock first so that only your requests move time. Commands use `curl` and a POSIX shell.

```sh
B=http://localhost:8080/inspected
curl -X PUT  -d '{"running":false}' $B/sim/clock
```

Payment gateway outage. The order fails after 3 failed calls, readiness is 503 and the metrics show the failures:

```sh
curl -X PUT  -d '{"mode":"outage"}' $B/sim/dependencies/payment-gateway
curl -X POST -d '{"sku":"sku-001","quantity":1}' $B/api/orders      # note the order id
curl -X POST -d '{"ticks":3}' $B/sim/advance
curl $B/api/orders/<id>                # failed, payment_gateway_unavailable
curl -i $B/health/ready                # 503, payment-gateway is in outage
curl -s $B/metrics | grep dependency_calls
```

Slow payment gateway. The order needs 3 ticks to be paid and readiness is degraded:

```sh
curl -X PUT  -d '{"mode":"slow"}' $B/sim/dependencies/payment-gateway
curl -X POST -d '{"sku":"sku-001","quantity":1}' $B/api/orders
curl -X POST -d '{"ticks":2}' $B/sim/advance    # still pending
curl -X POST -d '{"ticks":1}' $B/sim/advance    # paid
curl $B/health/ready                            # degraded, payment-gateway is slow
```

Stock depletion and restock. `sku-003` has 10 units; the 11th order fails with `out_of_stock`:

```sh
for i in 1 2 3 4 5 6 7 8 9 10 11; do curl -s -X POST -d '{"sku":"sku-003","quantity":1}' $B/api/orders; done
curl -X POST -d '{"ticks":2}' $B/sim/advance
curl $B/health/ready                            # degraded, out of stock: sku-003
curl -X POST -d '{"ticks":8}' $B/sim/advance    # tick 10 restocks sku-003
curl $B/health/ready                            # up
```

Resume the clock with `curl -X PUT -d '{"running":true}' $B/sim/clock`.
