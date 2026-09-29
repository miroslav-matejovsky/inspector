---
title: "01 - Fulfillment domain model"
dependencies: []
effort: "M"
complexity: "medium"
---

# 01 - Fulfillment domain model

## Objective

Package `harness/inspected/fulfillment` implements all business rules of the simulated order fulfillment service as a pure, deterministic state machine. It has no goroutines, no I/O, no wall-clock time, no randomness and no map iteration in any output path. Every rule in this step is covered by a unit test.

## Target Artifacts

| File | Content |
| --- | --- |
| `harness/inspected/fulfillment/doc.go` | Package doc: purpose, tick model, order lifecycle, all rule constants and their meaning. |
| `harness/inspected/fulfillment/catalog.go` | `SKU`, `Product`, `StandardCatalog`. |
| `harness/inspected/fulfillment/order.go` | `OrderID`, `OrderStatus`, `Channel`, `Reason`, `Transition`, `TransitionRule`, `OrderRequest`, `Order`, `AllowedTransitions`, `Channels`, `ParseOrderStatus`. |
| `harness/inspected/fulfillment/dependency.go` | `DependencyName`, `DependencyMode`, `Dependency`, `DependencyNames`, `DependencyModes`, `ParseDependencyMode`. |
| `harness/inspected/fulfillment/events.go` | `Event` and the four event types. |
| `harness/inspected/fulfillment/errors.go` | Sentinel errors. |
| `harness/inspected/fulfillment/state.go` | `Tick`, rule constants, `State`, `NewState`, `PlaceOrder`, `Tick`, `SetDependencyMode`, `Snapshot`. |
| `harness/inspected/fulfillment/snapshot.go` | `Snapshot`, `Snapshot.Product`, `Snapshot.Order`. |
| `harness/inspected/fulfillment/state_test.go` | Rule tests. |
| `harness/inspected/fulfillment/parse_test.go` | Parsing and enumeration tests. |
| `.go-arch-lint.yml` | Component `fulfillment`. |

## Implementation Tasks

1. Write `parse_test.go` and `state_test.go` with every test listed under Technical Details, Tests. Run `go test ./harness/inspected/fulfillment/...` and confirm it fails to compile.
2. Create `errors.go`, `catalog.go`, `order.go`, `dependency.go`, `events.go` with the types and functions below.
3. Create `state.go` and `snapshot.go` implementing the rules below.
4. Create `doc.go`.
5. In `.go-arch-lint.yml`, add under `components:` the line `fulfillment: { in: harness/inspected/fulfillment }`. Add no `deps` entry (it imports only stdlib).
6. Run `go test ./harness/inspected/fulfillment/...` until it passes, then `task all`.

## Technical Details

### Types

```go
// Tick is logical simulation time. The initial state is at tick 0.
type Tick uint64

type SKU string

type Product struct {
    SKU          SKU
    Name         string
    PriceCents   int64
    Stock        int
    Capacity     int
    ReorderPoint int
}

type OrderID string        // format "ord-%06d", sequence starts at 1
type OrderStatus string    // "pending", "paid", "shipped", "failed"
type Channel string        // "api", "simulation"
type Reason string         // see Reasons table

type Transition struct {
    From   OrderStatus // "" for the initial entry
    To     OrderStatus
    At     Tick
    Reason Reason
}

type TransitionRule struct {
    From   OrderStatus
    To     OrderStatus
    Reason Reason
}

type OrderRequest struct {
    SKU      SKU
    Quantity int
}

type Order struct {
    ID            OrderID
    SKU           SKU
    Quantity      int
    TotalCents    int64 // PriceCents * Quantity at placement
    Channel       Channel
    Status        OrderStatus
    PlacedAt      Tick
    UpdatedAt     Tick
    FailureReason Reason // empty unless Status == "failed"
    History       []Transition
}

type DependencyName string // "payment-gateway", "warehouse"
type DependencyMode string // "healthy", "slow", "outage"

type Dependency struct {
    Name DependencyName
    Mode DependencyMode
}
```

Exported constants for every enumeration value: `StatusPending`, `StatusPaid`, `StatusShipped`, `StatusFailed`, `ChannelAPI`, `ChannelSimulation`, `DependencyPaymentGateway`, `DependencyWarehouse`, `ModeHealthy`, `ModeSlow`, `ModeOutage`, and the `Reason...` constants below.

`func (s OrderStatus) Terminal() bool` returns true for `shipped` and `failed`.

### Reasons

| Constant | Value | Transition |
| --- | --- | --- |
| `ReasonOrderPlaced` | `order_placed` | `""` -> `pending` (history only, not a `TransitionRule`) |
| `ReasonPaymentAuthorized` | `payment_authorized` | `pending` -> `paid` |
| `ReasonPaymentDeclined` | `payment_declined` | `pending` -> `failed` |
| `ReasonPaymentGatewayUnavailable` | `payment_gateway_unavailable` | `pending` -> `failed` |
| `ReasonShipped` | `shipped` | `paid` -> `shipped` |
| `ReasonOutOfStock` | `out_of_stock` | `paid` -> `failed` |
| `ReasonWarehouseUnavailable` | `warehouse_unavailable` | `paid` -> `failed` |

`AllowedTransitions() []TransitionRule` returns the 6 rules of rows 2 to 7 in table order, as a new slice on every call.

### Enumerations and parsing

- `Channels() []Channel` returns `[api, simulation]`.
- `DependencyNames() []DependencyName` returns `[payment-gateway, warehouse]`.
- `DependencyModes() []DependencyMode` returns `[healthy, slow, outage]`.
- `ParseOrderStatus(s string) (OrderStatus, error)` accepts the 4 status values, otherwise returns an error wrapping `ErrInvalidOrderStatus`.
- `ParseDependencyMode(s string) (DependencyMode, error)` accepts the 3 mode values, otherwise returns an error wrapping `ErrInvalidDependencyMode`.

All slice-returning functions return a new slice on every call.

### Errors (`errors.go`)

`ErrInvalidCatalog`, `ErrUnknownProduct`, `ErrInvalidQuantity`, `ErrInvalidChannel`, `ErrUnknownDependency`, `ErrInvalidDependencyMode`, `ErrInvalidOrderStatus`, all created with `errors.New`. Returned errors wrap them with context, for example `fmt.Errorf("%w: %q", ErrUnknownProduct, sku)`.

### Rule constants (`state.go`)

| Constant | Value | Meaning |
| --- | --- | --- |
| `PaymentLimitCents` | `50000` | Payment is declined when `TotalCents > PaymentLimitCents`. |
| `MaxOrderQuantity` | `10` | Valid quantity is `1..MaxOrderQuantity`. |
| `HealthyLatencyTicks` | `1` | Ticks a stage waits before calling a `healthy` or `outage` dependency. |
| `SlowLatencyTicks` | `3` | Ticks a stage waits before calling a `slow` dependency. |
| `MaxStageAttempts` | `3` | Failed dependency calls per stage before the order fails. |
| `RestockIntervalTicks` | `10` | Restock runs when `now % RestockIntervalTicks == 0`. |
| `MaxRetainedOrders` | `1000` | Upper bound of retained orders; oldest terminal orders are pruned first. |

### Standard catalog

`StandardCatalog() []Product` returns a new slice:

| SKU | Name | PriceCents | Stock | Capacity | ReorderPoint |
| --- | --- | ---: | ---: | ---: | ---: |
| `sku-001` | Mechanical Keyboard | 8900 | 40 | 40 | 10 |
| `sku-002` | USB-C Dock | 14900 | 20 | 20 | 5 |
| `sku-003` | 27in Monitor | 32900 | 10 | 10 | 3 |
| `sku-004` | Webcam | 5900 | 30 | 30 | 8 |
| `sku-005` | Desk Lamp | 3900 | 50 | 50 | 10 |

### Events (`events.go`)

```go
// Event is a fact produced by a State transition. The set is closed.
type Event interface{ isEvent() }

type OrderPlaced struct {
    Order    OrderID
    SKU      SKU
    Quantity int
    Channel  Channel
    At       Tick
}

type OrderStatusChanged struct {
    Order    OrderID
    SKU      SKU
    From     OrderStatus
    To       OrderStatus
    Reason   Reason
    PlacedAt Tick
    At       Tick
}

type DependencyCalled struct {
    Dependency DependencyName
    Succeeded  bool
    At         Tick
}

type ProductRestocked struct {
    SKU  SKU
    From int
    To   int
    At   Tick
}
```

Each type implements `isEvent()` with a value receiver.

### State (`state.go`)

```go
type State struct {
    now          Tick
    products     []Product      // sorted by SKU
    productIndex map[SKU]int    // lookup only, never iterated
    orders       []*orderRecord // placement order
    nextOrder    uint64
    dependencies []Dependency   // sorted by Name
}

type orderRecord struct {
    order     Order
    waitTicks int // ticks left before the current stage calls its dependency
    attempts  int // failed dependency calls in the current stage
}

func NewState(catalog []Product) (*State, error)
func (s *State) PlaceOrder(req OrderRequest, channel Channel) (Order, []Event, error)
func (s *State) Tick() []Event
func (s *State) SetDependencyMode(name DependencyName, mode DependencyMode) error
func (s *State) Snapshot() Snapshot
```

`NewState` copies the catalog, sorts it by SKU and sets both dependencies to `healthy`. It returns an error wrapping `ErrInvalidCatalog` when: the catalog is empty; a SKU is empty or duplicated; a name is empty; `PriceCents <= 0`; `Capacity <= 0`; `Stock < 0` or `Stock > Capacity`; `ReorderPoint < 0` or `ReorderPoint >= Capacity`.

`latency(mode)` returns `SlowLatencyTicks` for `slow`, otherwise `HealthyLatencyTicks`.

`PlaceOrder`:

1. Validate: channel is one of `Channels()` (else `ErrInvalidChannel`), SKU exists (else `ErrUnknownProduct`), `1 <= Quantity <= MaxOrderQuantity` (else `ErrInvalidQuantity`). On error, state is unchanged.
2. `nextOrder++`, `ID = fmt.Sprintf("ord-%06d", nextOrder)`, `TotalCents = PriceCents * Quantity`.
3. Status `pending`, `PlacedAt = UpdatedAt = now`, `History = [{"" -> pending, now, order_placed}]`.
4. `waitTicks = latency(mode(payment-gateway))`, `attempts = 0`.
5. Return a deep copy of the order and `[OrderPlaced{...}]`.

Stock is not checked or reserved at placement.

`Tick`:

```text
now++
for each order record in placement order, skipping terminal orders:
    waitTicks--
    if waitTicks > 0: continue
    pending -> processPayment
    paid    -> processShipment
if now % RestockIntervalTicks == 0 and mode(warehouse) != outage:
    for each product in SKU order with Stock <= ReorderPoint:
        emit ProductRestocked{SKU, From: Stock, To: Capacity, At: now}; Stock = Capacity
prune
return events in the order they were produced
```

`processPayment(r)`:

```text
if mode(payment-gateway) == outage:
    emit DependencyCalled{payment-gateway, false, now}
    attempts++
    if attempts >= MaxStageAttempts: transition(r, failed, payment_gateway_unavailable)
    else: waitTicks = HealthyLatencyTicks
    return
emit DependencyCalled{payment-gateway, true, now}
if TotalCents > PaymentLimitCents: transition(r, failed, payment_declined); return
transition(r, paid, payment_authorized)
attempts = 0
waitTicks = latency(mode(warehouse))
```

`processShipment(r)`: same retry pattern against `warehouse` with reason `warehouse_unavailable`. On a successful call: if `product.Stock < Quantity` then `transition(r, failed, out_of_stock)`, else `product.Stock -= Quantity` and `transition(r, shipped, shipped)`.

`transition(r, to, reason)`: panics with a message naming from, to and reason when the triple is not in `AllowedTransitions()` (programming error, fail fast). Otherwise sets `Status = to`, `UpdatedAt = now`, `FailureReason = reason` when `to == failed`, appends `Transition{from, to, now, reason}` to `History` and emits `OrderStatusChanged`.

`prune`: while `len(orders) > MaxRetainedOrders`, remove the oldest terminal order. Stop when no terminal order is left. Active orders are never removed. Remaining orders keep placement order.

`SetDependencyMode`: returns an error wrapping `ErrUnknownDependency` for an unknown name and `ErrInvalidDependencyMode` for a mode not in `DependencyModes()`. Setting the current mode again is a no-op without error. The new mode affects the next dependency call and the latency of the next stage entry; running waits are not recalculated.

### Snapshot (`snapshot.go`)

```go
type Snapshot struct {
    Now          Tick
    Products     []Product    // SKU order
    Orders       []Order      // placement order, History deep-copied
    Dependencies []Dependency // name order
}

func (s Snapshot) Product(sku SKU) (Product, bool)
func (s Snapshot) Order(id OrderID) (Order, bool)
```

A snapshot shares no memory with the `State`.

### Tests

All tests use `require`. All start from `NewState(StandardCatalog())` unless stated.

`parse_test.go`:

| Test | Assertion |
| --- | --- |
| `TestParseOrderStatus` | Table: 4 valid values parse; `""`, `"PAID"`, `"unknown"` return `ErrInvalidOrderStatus`. |
| `TestParseDependencyMode` | Table: 3 valid values parse; `""`, `"down"` return `ErrInvalidDependencyMode`. |
| `TestAllowedTransitions` | Returns exactly the 6 rules in table order. |
| `TestStandardCatalog` | 5 products, SKUs `sku-001`..`sku-005`, `NewState` accepts it. |

`state_test.go`:

| Test | Setup | Assertion |
| --- | --- | --- |
| `TestNewStateRejectsInvalidCatalog` | Table: empty, duplicate SKU, empty SKU, empty name, zero price, zero capacity, negative stock, stock > capacity, reorder point >= capacity | `ErrorIs(err, ErrInvalidCatalog)` |
| `TestNewStateCopiesCatalog` | Mutate the input slice after `NewState` | `Snapshot().Products[0].Stock == 40` |
| `TestPlaceOrderValidation` | Table: SKU `sku-999`; quantity 0; quantity 11; channel `"x"` | `ErrUnknownProduct`, `ErrInvalidQuantity`, `ErrInvalidQuantity`, `ErrInvalidChannel`; `Snapshot().Orders` is empty |
| `TestPlaceOrderCreatesPendingOrder` | `sku-001` x2, `api` | ID `ord-000001`, `TotalCents` 17800, `pending`, `PlacedAt` 0, history `[{"", pending, 0, order_placed}]`, events `[OrderPlaced{ord-000001, sku-001, 2, api, 0}]` |
| `TestHappyPath` | `sku-001` x2, then `Tick()` twice | Tick 1 events `[DependencyCalled{payment-gateway, true, 1}, OrderStatusChanged{pending->paid, payment_authorized, PlacedAt 0, At 1}]`; tick 2 events `[DependencyCalled{warehouse, true, 2}, OrderStatusChanged{paid->shipped, shipped, At 2}]`; stock of `sku-001` is 38; history has 3 entries |
| `TestPaymentDeclined` | `sku-003` x2 (65800), `Tick()` | `failed`, `FailureReason` `payment_declined`, stock of `sku-003` still 10 |
| `TestPaymentGatewayOutage` | Mode `outage`, `sku-001` x1, 3 ticks | `pending` after ticks 1 and 2; `failed` with `payment_gateway_unavailable` at tick 3; exactly 3 `DependencyCalled{payment-gateway, false}` events |
| `TestPaymentGatewayRecoversBeforeLastAttempt` | Mode `outage`, place, tick 1, mode `healthy`, tick 2 | `paid` at tick 2 |
| `TestSlowPaymentGateway` | Mode `slow`, `sku-001` x1, 4 ticks | `pending` after ticks 1 and 2, `paid` at tick 3, `shipped` at tick 4 |
| `TestWarehouseOutage` | Warehouse `outage`, `sku-001` x1, 4 ticks | `paid` at tick 1, still `paid` after ticks 2 and 3, `failed` with `warehouse_unavailable` at tick 4 |
| `TestOutOfStock` | 11 orders `sku-003` x1 at tick 0, 2 ticks | `ord-000001`..`ord-000010` `shipped`, `ord-000011` `failed` with `out_of_stock`, stock 0 |
| `TestRestock` | Continue `TestOutOfStock` setup to tick 10 | Tick 10 emits exactly one `ProductRestocked{sku-003, 0, 10, 10}`; stock 10; ticks 3..9 emit no `ProductRestocked` |
| `TestRestockSkippedDuringWarehouseOutage` | Out-of-stock setup, warehouse `outage` before tick 10, tick to 10, mode `healthy`, tick to 20 | No restock at tick 10 (stock 0); `ProductRestocked{sku-003, 0, 10, 20}` at tick 20 |
| `TestSetDependencyMode` | Table | Unknown name returns `ErrUnknownDependency`; mode `"down"` returns `ErrInvalidDependencyMode`; valid change is visible in `Snapshot().Dependencies`; repeated same mode returns nil |
| `TestPruneRemovesOldestTerminalOrders` | 1000 orders `sku-005` x1, 2 ticks (all terminal), 5 more orders, 1 tick | 1000 retained orders; first retained ID `ord-000006`; `ord-001001`..`ord-001005` present |
| `TestPruneKeepsActiveOrders` | 1001 orders `sku-005` x1, 1 tick | 1001 retained orders, all `paid` |
| `TestSnapshotIsDeepCopy` | Take snapshot, mutate `Products[0].Stock`, `Orders[0].History[0].Reason`, `Dependencies[0].Mode` | A new snapshot is unchanged |
| `TestSnapshotLookup` | 1 order | `Product("sku-001")` found, `Product("sku-999")` not found, `Order("ord-000001")` found, `Order("ord-999999")` not found |
| `TestStateIsDeterministic` | Two states, same sequence: 20 orders over 3 SKUs, mode changes, 25 ticks | `require.Equal` on both snapshots and on both collected event slices |

## Verification

```powershell
go test ./harness/inspected/fulfillment/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/inspected/fulfillment/...` exits 0 and runs every test listed above.
- `harness/inspected/fulfillment` imports only standard library packages (`go list -f "{{.Imports}}" ./harness/inspected/fulfillment` prints only stdlib paths).
- No file in the package imports `time`, `math/rand`, `math/rand/v2`, `sync` or `net`.
- `.go-arch-lint.yml` contains component `fulfillment`, and `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Concurrency safety. `State` is owned by exactly one goroutine (step 02).
- Random traffic. The domain never generates orders by itself.
- Order cancellation, refunds, multi-line orders, customer accounts.
- JSON tags. JSON contracts live in `harness/inspected` (steps 05, 06).
