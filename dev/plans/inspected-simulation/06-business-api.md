---
title: "06 - Business API"
dependencies: ["05-inspected-app"]
effort: "M"
complexity: "medium"
---

# 06 - Business API

## Objective

The inspected service exposes its products and orders as JSON resources under `/inspected/api`. Orders can be placed through the API. Every resource carries `links` to related resources. End-to-end scenario tests prove that an injected cause (outage, slow dependency, stock depletion) is visible consistently in the API, health and metrics.

## Target Artifacts

| File | Content |
| --- | --- |
| `harness/inspected/api.go` | Handlers: list products, get product, list orders, get order, place order. |
| `harness/inspected/routes.go` | 5 new routes. |
| `harness/inspected/resources.go` | Product, order and list resources and converters. |
| `harness/inspected/respond.go` | New rows in `errorResponse`. |
| `harness/inspected/observability.go` | Index gets `products` and `orders` links. |
| `harness/inspected/doc.go` | Endpoint table and error codes updated. |
| `harness/inspected/api_test.go` | API tests. |
| `harness/inspected/scenario_test.go` | End-to-end scenario tests. |

## Implementation Tasks

1. Write `api_test.go` and `scenario_test.go` with every test listed below. Confirm they fail.
2. Add resource types and converters to `resources.go`.
3. Create `api.go` with the 5 handlers.
4. Register the routes in `routes.go`.
5. Add the error rows to `errorResponse` in `respond.go`.
6. Add the `products` and `orders` links to `handleIndex`.
7. Update `doc.go`.
8. Run `go test ./harness/inspected/...` until it passes, then `task all`.

## Technical Details

### Routes

| Pattern | Route name | Handler |
| --- | --- | --- |
| `GET /inspected/api/products` | `products` | `handleListProducts` |
| `GET /inspected/api/products/{sku}` | `product` | `handleGetProduct` |
| `GET /inspected/api/orders` | `orders` | `handleListOrders` |
| `GET /inspected/api/orders/{id}` | `order` | `handleGetOrder` |
| `POST /inspected/api/orders` | `place_order` | `handlePlaceOrder` |

### Resources

```go
type productResource struct {
    SKU          string       `json:"sku"`
    Name         string       `json:"name"`
    PriceCents   int64        `json:"price_cents"`
    Stock        int          `json:"stock"`
    Capacity     int          `json:"capacity"`
    ReorderPoint int          `json:"reorder_point"`
    Links        productLinks `json:"links"`
}

type productLinks struct {
    Self   string `json:"self"`   // /inspected/api/products/{sku}
    Orders string `json:"orders"` // /inspected/api/orders?sku={sku}
}

type orderResource struct {
    ID            string               `json:"id"`
    SKU           string               `json:"sku"`
    Quantity      int                  `json:"quantity"`
    TotalCents    int64                `json:"total_cents"`
    Channel       string               `json:"channel"`
    Status        string               `json:"status"`
    PlacedAtTick  uint64               `json:"placed_at_tick"`
    UpdatedAtTick uint64               `json:"updated_at_tick"`
    FailureReason string               `json:"failure_reason,omitempty"`
    History       []transitionResource `json:"history"`
    Links         orderLinks           `json:"links"`
}

type transitionResource struct {
    From   string `json:"from,omitempty"` // omitted for the initial entry
    To     string `json:"to"`
    AtTick uint64 `json:"at_tick"`
    Reason string `json:"reason"`
}

type orderLinks struct {
    Self    string `json:"self"`    // /inspected/api/orders/{id}
    Product string `json:"product"` // /inspected/api/products/{sku}
}

type productListResource struct {
    Products []productResource `json:"products"`
}

type orderListResource struct {
    Orders []orderResource `json:"orders"` // never null, empty list is []
}

type placeOrderRequest struct {
    SKU      string `json:"sku"`
    Quantity int    `json:"quantity"`
}
```

Path segments are escaped with `url.PathEscape`, query values with `url.QueryEscape`.

### Handlers

| Handler | Behavior |
| --- | --- |
| `handleListProducts` | `a.sim.Snapshot(ctx)`; 200 `productListResource` in SKU order. |
| `handleGetProduct` | Snapshot; `Snapshot.Product(SKU(r.PathValue("sku")))`; missing is 404 `product_not_found`. |
| `handleListOrders` | Query `status` (optional, parsed with `fulfillment.ParseOrderStatus`, invalid is 400 `invalid_status`) and `sku` (optional, exact match, unknown SKU gives an empty list). Snapshot; filter; 200 `orderListResource` in placement order. |
| `handleGetOrder` | Snapshot; `Snapshot.Order(OrderID(r.PathValue("id")))`; missing is 404 `order_not_found`. |
| `handlePlaceOrder` | Decode `placeOrderRequest`; `a.sim.PlaceOrder(ctx, OrderRequest{...})`; 201 with header `Location` = order self link and body `orderResource`. |

### New `errorResponse` rows

| Error | Status | Code |
| --- | --- | --- |
| `fulfillment.ErrUnknownProduct` | 422 | `unknown_product` |
| `fulfillment.ErrInvalidQuantity` | 422 | `invalid_quantity` |
| `fulfillment.ErrInvalidOrderStatus` | 400 | `invalid_status` |

`product_not_found` and `order_not_found` are written directly by their handlers.

### Tests

Default config is `testConfig()` from step 05 (`OrdersPerTick: 0`, clock never fires). Time moves only through `POST /inspected/sim/advance`.

`api_test.go`:

| Test | Assertion |
| --- | --- |
| `TestListProducts` | 200; 5 products; first is `{"sku":"sku-001","name":"Mechanical Keyboard","price_cents":8900,"stock":40,"capacity":40,"reorder_point":10,"links":{"self":"/inspected/api/products/sku-001","orders":"/inspected/api/orders?sku=sku-001"}}`. |
| `TestGetProduct` | `sku-002` is 200 with name `USB-C Dock`; `sku-999` is 404 `product_not_found`. |
| `TestPlaceOrder` | `POST` `{"sku":"sku-001","quantity":2}` is 201; `Location` is `/inspected/api/orders/ord-000001`; body has `status` `pending`, `total_cents` 17800, `channel` `api`, `placed_at_tick` 0, `history` `[{"to":"pending","at_tick":0,"reason":"order_placed"}]`, links `self` and `product`. |
| `TestPlaceOrderRejectsInvalidRequest` | Table: `{"sku":"sku-999","quantity":1}` 422 `unknown_product`; quantity 0 and 11 are 422 `invalid_quantity`; `{` 400; `{"sku":"sku-001","quantity":1,"x":1}` 400; empty body 400. No order exists afterwards (`GET /inspected/api/orders` returns `{"orders":[]}`). |
| `TestGetOrder` | Place `sku-001` x1, advance 2; order is `shipped` with 3 history entries (`order_placed`, `payment_authorized`, `shipped`) at ticks 0, 1, 2; `ord-999999` is 404 `order_not_found`. |
| `TestListOrdersEmpty` | Body is exactly `{"orders":[]}` followed by a newline. |
| `TestListOrdersFilters` | Place `sku-001` x2 and `sku-003` x2, advance 1. No filter gives 2 orders; `?status=failed` gives only the `sku-003` order; `?sku=sku-001` gives only the `sku-001` order; `?status=paid&sku=sku-003` gives 0; `?status=bogus` is 400 `invalid_status`. |
| `TestIndexListsBusinessLinks` | Index links contain `products` = `/inspected/api/products` and `orders` = `/inspected/api/orders`; `TestIndexLinksResolve` from step 05 covers them. |
| `TestOrdersMethodNotAllowed` | `DELETE /inspected/api/orders` is 405. |

`scenario_test.go`:

| Test | Steps | Assertions |
| --- | --- | --- |
| `TestScenarioPaymentGatewayOutage` | Set `payment-gateway` `outage`; place `sku-001` x1; advance 3 | Order `failed`, `failure_reason` `payment_gateway_unavailable`; ready 503 with the `payment-gateway` check `down`; metrics lines `fulfillment_dependency_calls_total{dependency="payment-gateway",result="failure"} 3` and `fulfillment_order_transitions_total{from="pending",reason="payment_gateway_unavailable",to="failed"} 1`, `fulfillment_dependency_up{dependency="payment-gateway"} 0`. |
| `TestScenarioSlowPaymentGateway` | Set `payment-gateway` `slow`; place `sku-001` x1; advance 2, then 1, then 1 | `pending` after 2, `paid` after 3, `shipped` after 4; ready 200 with status `degraded` during the scenario. |
| `TestScenarioOutOfStockAndRestock` | Place 11 orders `sku-003` x1; advance 2 | 10 orders `shipped`, `ord-000011` `failed` with `out_of_stock`; product `sku-003` stock 0; ready 200 status `degraded` with inventory reason `out of stock: sku-003`; metrics line `fulfillment_product_stock{sku="sku-003"} 0`. Then advance 8 (tick 10): stock 10; ready status `up`; metrics line `fulfillment_product_restocks_total{sku="sku-003"} 1`. |
| `TestScenarioWarehouseOutage` | Set `warehouse` `outage`; place `sku-001` x1; advance 4 | Order `failed` with `warehouse_unavailable`; history ticks 0, 1, 4. |
| `TestScenarioSimulatedTrafficIsReproducible` | Two apps, config `Seed 7`, `OrdersPerTick 2`; each advances 15 | `GET /inspected/api/orders` bodies are byte-equal; the lines starting with `fulfillment_` in both metrics bodies are equal; 30 orders exist. |

## Verification

```powershell
go test ./harness/inspected/...
task all
```

## Acceptance Criteria

- `go test ./harness/inspected/...` exits 0 and runs every test listed in steps 05 and 06.
- `GET /inspected/api/orders` with no orders returns exactly `{"orders":[]}` plus newline (`TestListOrdersEmpty`).
- Every product and order resource in test responses has non-empty `links` pointing under `/inspected/`.
- `task all` exits 0.

## Non-Goals

- Pagination, sorting options, field selection.
- Order cancellation or modification.
- Placing orders through the control API.
- Links to health checks or metrics from business resources.
