# Harness

Development tooling for running Inspector against something to inspect.

## workbench

`workbench/` is a Go package that serves a web page with two panels: **inspected** and **inspector**. Both panels are empty for now. It also mounts the inspected simulation under `/inspected/` on the same server.

The entry point is `cmd/workbench`. Start it with `task workbench` and open http://localhost:8080. Every setting is a required flag without a default:

| Flag | Meaning |
| --- | --- |
| `-addr` | Listen address, for example `localhost:8080`. |
| `-inspected-seed` | Seed of the traffic generator. The same seed produces the same traffic. |
| `-inspected-orders-per-tick` | Simulated orders placed per tick, 0 to 100. |
| `-inspected-tick-interval` | Wall-clock time between simulation ticks, for example `1s`. |
| `-inspected-request-timeout` | Max wait for the simulation per HTTP request, for example `2s`. |
| `-inspector-source-timeout` | Max wait for one read of the inspected service by the inspector, for example `2s`. |

The workbench also mounts the Inspector views of the simulation under `/inspector/`. They are built from the library packages `connectivity`, `representation` and the harness package `adapter`, and read the inspected service over HTTP through the workbench listener.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/inspector/` | Overview: observation time, entity kinds with counts |
| GET | `/inspector/entities` | Entities, optional filter `kind` |
| GET | `/inspector/entities/{kind}/{id}` | One entity with attributes and related entities |
| GET | `/inspector/entities/{kind}/{id}/explanation` | Why the entity is in its state: cause tree and root causes |

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

## adapter

`adapter/` maps the inspected simulation into the Inspector observation model. It is the only package that knows both the inspected JSON contract and the library model; all simulated vocabulary Inspector shows is defined here. It reads the inspected service like an external client: GET requests to `/inspected/health/ready`, `/inspected/api/dependencies`, `/inspected/api/products` and `/inspected/api/orders` through `connectivity.HTTPReader`. It never reads `/inspected/sim` and does not import package `inspected`.

Entities:

| Source | Kind | ID | State | Reason | History | Attributes |
| --- | --- | --- | --- | --- | --- | --- |
| readiness | `service` | `inspected` | readiness `status` | none | none | none |
| each check | `health_check` | `name` | `status` | check `reason` | none | none |
| each dependency | `dependency` | `name` | `mode` | none | none | none |
| each product | `product` | `sku` | empty | none | none | `name`, `price_cents`, `stock`, `capacity`, `reorder_point` |
| each order | `order` | `id` | `status` | reason of the last history entry | source `history`, `at` written as `tick <n>` | `quantity`, `total_cents`, `channel`, `placed_at_tick`, `updated_at_tick` |

Relations:

| From | Kind | To | Cause |
| --- | --- | --- | --- |
| `service/inspected` | `has_check` | `health_check/<name>`, one per check | when the check status equals the service status |
| `health_check/<name>` | `caused_by` | each link in the check `causes` | yes |
| `order/<id>` | `for_product` | `product/<sku>`, one per order | no |
| `order/<id>` | `caused_by` | the `cause` link of the last history entry | yes |
| `order/<id>` | `waits_on` | the `links.waiting_on` of an open order | yes |

The service status is the worst check status, so the checks with that status are the ones that decide it. A link ending in `/api/dependencies/<name>` targets `dependency/<name>`, one ending in `/api/products/<sku>` targets `product/<sku>`; any other link becomes a gap.

The four reads are not atomic; they can come from different ticks.

Failed reads and malformed items become gaps of the snapshot and are shown in every inspector view, for example `unexpected status 503: simulation_unavailable` for `/api/products`. The other reads are still shown. Only when no read succeeds do the views answer 503.
