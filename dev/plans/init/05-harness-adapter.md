---
title: "05 - Harness adapter"
dependencies: ["02-observation-model", "04-connectivity-http-reader"]
effort: "M"
complexity: "medium"
---

# 05 - Harness adapter

## Objective

Package `harness/adapter` observes the inspected simulation through its read-only HTTP API and maps what it reads into an `observation.Snapshot`. It is the only package that knows both the inspected JSON contract and the Inspector model. All simulated domain vocabulary that Inspector shows (kinds, relation names, attribute names) is defined here, in the harness, never in the library. The adapter imports neither `harness/inspected` nor any other harness package in production code.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/adapter/doc.go` | New: package documentation with the mapping tables. |
| `harness/adapter/adapter.go` | New: `Adapter`, `New`, `Adapter.Observe`, kind and relation constants, mapping functions. |
| `harness/adapter/contract.go` | New: unexported JSON structs of the three inspected documents. |
| `harness/adapter/adapter_test.go` | New tests with canned JSON, package `adapter_test`. |
| `harness/adapter/contract_test.go` | New contract test against the real inspected app, package `adapter_test`. |
| `.go-arch-lint.yml` | Component `adapter: { in: harness/adapter }`; `deps`: `adapter: { mayDependOn: [observation, connectivity] }`. |
| `harness/README.md` | New section `## adapter`. |
| `README.md` (root) | One sentence in `## Library`: the harness maps the inspected simulation into the library model in `harness/adapter`. |

## Implementation Tasks

1. Write `adapter_test.go` and `contract_test.go` with every test listed below. Confirm they fail to compile.
2. Create `contract.go`, then `adapter.go`.
3. Create `doc.go`.
4. Add the component and the rule to `.go-arch-lint.yml`.
5. Add the `## adapter` section to `harness/README.md` and the sentence to root `README.md`.
6. Run `go test ./harness/adapter/...` until it passes, then `task all`.

## Technical Details

### API (`adapter.go`)

```go
// Adapter observes the inspected service through its read-only HTTP API.
type Adapter struct {
    reader *connectivity.HTTPReader // base URL is the inspected path prefix, for example http://127.0.0.1:8080/inspected
    now    func() time.Time
}

// New returns an adapter that reads with reader and stamps each snapshot with
// now. Both are required.
func New(reader *connectivity.HTTPReader, now func() time.Time) (*Adapter, error)

// Observe reads readiness, products and orders, in this order, and returns
// them as one snapshot stamped with now() after the last read. The three
// reads are not atomic: they can reflect different simulation ticks.
func (a *Adapter) Observe(ctx context.Context) (observation.Snapshot, error)
```

`New` errors: `adapter: reader is required`, `adapter: clock is required`.

Constants (unexported):

```go
const (
    kindService     = "service"
    kindHealthCheck = "health_check"
    kindProduct     = "product"
    kindOrder       = "order"

    relationHasCheck   = "has_check"
    relationForProduct = "for_product"

    serviceID = "inspected" // the inspected service is the only service

    pathReadiness = "/health/ready"
    pathProducts  = "/api/products"
    pathOrders    = "/api/orders"
)
```

### Reads

| Path | Accepted statuses | Decoded into |
| --- | --- | --- |
| `/health/ready` | 200, 503 (readiness down is an observation) | `healthDoc` |
| `/api/products` | 200 | `productListDoc` |
| `/api/orders` | 200 | `orderListDoc` |

Any other status: `adapter: read %s: unexpected status %d` with the path. Reader and decode errors: `adapter: read %s: %w`.

### Contract (`contract.go`)

Own copies of the inspected JSON resources, only the fields the adapter uses. Unknown fields are ignored by `Document.Decode`.

```go
type healthDoc struct {
    Status string     `json:"status"`
    Checks []checkDoc `json:"checks"`
}

type checkDoc struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Reason string `json:"reason"`
}

type productListDoc struct {
    Products []productDoc `json:"products"`
}

type productDoc struct {
    SKU          string `json:"sku"`
    Name         string `json:"name"`
    PriceCents   int64  `json:"price_cents"`
    Stock        int    `json:"stock"`
    Capacity     int    `json:"capacity"`
    ReorderPoint int    `json:"reorder_point"`
}

type orderListDoc struct {
    Orders []orderDoc `json:"orders"`
}

type orderDoc struct {
    ID            string `json:"id"`
    SKU           string `json:"sku"`
    Quantity      int    `json:"quantity"`
    TotalCents    int64  `json:"total_cents"`
    Channel       string `json:"channel"`
    Status        string `json:"status"`
    PlacedAtTick  uint64 `json:"placed_at_tick"`
    UpdatedAtTick uint64 `json:"updated_at_tick"`
    FailureReason string `json:"failure_reason"`
}
```

Required fields. A missing one fails with `adapter: read %s: %s %d without %s` (path, item, zero-based index, JSON field name), for example `adapter: read /api/orders: order 3 without sku`:

| Item | Required |
| --- | --- |
| readiness | `status` |
| check | `name`, `status` |
| product | `sku` |
| order | `id`, `sku`, `status` |

### Mapping

Entities, in this order: service, checks (source order), products (source order), orders (source order). Numbers are formatted with `strconv`. An entity without attributes has `Attributes == nil`.

| Source | Kind | ID | State | Attributes, in this order |
| --- | --- | --- | --- | --- |
| readiness | `service` | `inspected` | readiness `status` | none |
| each check | `health_check` | `name` | `status` | `reason`, only when not empty |
| each product | `product` | `sku` | empty | `name`, `price_cents`, `stock`, `capacity`, `reorder_point` |
| each order | `order` | `id` | `status` | `quantity`, `total_cents`, `channel`, `placed_at_tick`, `updated_at_tick`, `failure_reason` only when not empty |

Relations, in this order:

| Relation | From | Kind | To |
| --- | --- | --- | --- |
| per check | `service/inspected` | `has_check` | `health_check/<name>` |
| per order | `order/<id>` | `for_product` | `product/<sku>` |

The order's `sku` is expressed only as the `for_product` relation, not as an attribute.

Finally `observation.NewSnapshot(a.now(), entities, relations)`; an error is returned as `adapter: %w` (it still wraps `observation.ErrInvalidSnapshot`).

### `doc.go`

1. Purpose: maps the inspected simulation into the Inspector observation model; the only package that knows both.
2. `# Boundary`: reads only `/health/ready`, `/api/products`, `/api/orders` with GET through `connectivity.HTTPReader`; no Go import of `harness/inspected`; never reads `/sim`.
3. `# Mapping`: both tables above.
4. `# Consistency`: the three reads are not atomic.

### `harness/README.md`

New section `## adapter` after `## inspected`: one paragraph on purpose and boundary, and the two mapping tables.

### Tests (`adapter_test.go`, package `adapter_test`)

A helper `source(t, docs map[string]doc)` starts `httptest.NewServer` with a `http.ServeMux` that serves `GET /inspected<path>` with the given status and JSON body, and returns an `*adapter.Adapter` whose reader base is `srv.URL + "/inspected"` (timeout 1 second) and whose clock returns `time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)`.

Default documents:

```json
// /inspected/health/ready, status 200
{"status":"degraded","checks":[
  {"name":"payment-gateway","status":"degraded","reason":"payment-gateway is slow: calls take 3 ticks"},
  {"name":"inventory","status":"up"}]}

// /inspected/api/products, status 200
{"products":[{"sku":"sku-001","name":"Mechanical Keyboard","price_cents":8900,"stock":40,
  "capacity":40,"reorder_point":10,"links":{"self":"/inspected/api/products/sku-001"}}]}

// /inspected/api/orders, status 200
{"orders":[{"id":"ord-000001","sku":"sku-001","quantity":2,"total_cents":17800,"channel":"api",
  "status":"failed","placed_at_tick":1,"updated_at_tick":4,"failure_reason":"payment_gateway_unavailable",
  "history":[],"links":{"self":"/inspected/api/orders/ord-000001"}}]}
```

| Test | Assertion |
| --- | --- |
| `TestObserveMapsEntities` | `Entities()` equals exactly: `service/inspected` state `degraded`, no attributes; `health_check/payment-gateway` state `degraded`, `[reason=payment-gateway is slow: calls take 3 ticks]`; `health_check/inventory` state `up`, no attributes; `product/sku-001` no state, `[name=Mechanical Keyboard, price_cents=8900, stock=40, capacity=40, reorder_point=10]`; `order/ord-000001` state `failed`, `[quantity=2, total_cents=17800, channel=api, placed_at_tick=1, updated_at_tick=4, failure_reason=payment_gateway_unavailable]`. |
| `TestObserveMapsRelations` | `Relations()` equals exactly `[service/inspected -has_check-> health_check/payment-gateway, service/inspected -has_check-> health_check/inventory, order/ord-000001 -for_product-> product/sku-001]`. |
| `TestObserveStampsTime` | `ObservedAt()` equals the fixed clock time. |
| `TestObserveAcceptsUnavailableReadiness` | Readiness status 503 with `{"status":"down","checks":[{"name":"warehouse","status":"down","reason":"warehouse is in outage: calls fail"}]}`: no error; `service/inspected` state `down`. |
| `TestObserveRejectsUnexpectedStatus` | Products answer 500 `{"error":{"code":"internal_error","message":"x"}}`: error contains `/api/products` and `500`. |
| `TestObserveRejectsMissingField` | Table: check without `name`, product without `sku`, order without `id`, order without `sku`, order without `status`, readiness without `status`. Each returns an error containing the expected `without <field>` text. |
| `TestObserveRejectsUnknownProduct` | Order with `sku` `sku-999`: `require.ErrorIs(err, observation.ErrInvalidSnapshot)`. |
| `TestObserveFailsWhenSourceIsDown` | Server closed before `Observe`: error contains `/health/ready`. |
| `TestNewRejectsMissingDependencies` | `New(nil, time.Now)` and `New(reader, nil)` return errors. |

### Contract test (`contract_test.go`, package `adapter_test`)

Test files are excluded from go-arch-lint, so this test may import `harness/inspected`.

| Test | Assertion |
| --- | --- |
| `TestObserveInspectedService` | `inspected.New(inspected.Config{Seed: 1, OrdersPerTick: 0, TickInterval: time.Hour, RequestTimeout: time.Second})`; `app.Run(ctx)` in a goroutine, cancelled on cleanup, must return nil. `srv := httptest.NewServer(app.Handler())`. `POST srv.URL + "/inspected/api/orders"` with `{"sku":"sku-001","quantity":1}` returns 201. Adapter with reader base `srv.URL + inspected.PathPrefix`. `Observe` returns: `service/inspected` state `up`; `health_check` entities `payment-gateway`, `warehouse`, `inventory`, each state `up`; `product` entities `sku-001` to `sku-005`; `product/sku-001` attribute `stock` is `40`; exactly one `order` entity, state `pending`; relation `order/<id> -for_product-> product/sku-001` where `<id>` is the `id` of the POST response. |

## Verification

```powershell
go test ./harness/adapter/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./harness/adapter/...` exits 0 and runs every test listed above.
- `go list -f '{{.Imports}}' ./harness/adapter` prints no path containing `harness/`.
- `.go-arch-lint.yml` rule for `adapter` is exactly `mayDependOn: [observation, connectivity]`.
- `harness/README.md` contains the section `## adapter` with both mapping tables.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Order history, metrics and the `/inspected/sim` endpoints.
- Relations the API does not state, for example which dependency an order is waiting on.
- Derived states, for example "out of stock" for a product. The adapter maps facts only.
- Caching or polling.
