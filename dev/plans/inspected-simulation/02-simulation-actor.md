---
title: "02 - Simulation actor"
dependencies: ["01-fulfillment-domain"]
effort: "M"
complexity: "medium"
---

# 02 - Simulation actor

## Objective

Package `harness/inspected/simulation` provides `Simulation`, the single owner of the `fulfillment.State`. It is an actor built on plain Go: one goroutine (`Run`) owns all mutable state and processes typed messages from an unbuffered mailbox. It generates seeded, reproducible traffic, consumes an injected clock channel and reports events and snapshots to a `Recorder`. All behavior is tested without timers or sleeps.

## Target Artifacts

| File | Content |
| --- | --- |
| `harness/inspected/simulation/doc.go` | Package doc: ownership, lifecycle, message contracts, failure boundary, restart assumptions, determinism guarantee. |
| `harness/inspected/simulation/simulation.go` | `Config`, `Recorder`, `Status`, `Simulation`, `New`, `Run`, public request methods, errors, constants. |
| `harness/inspected/simulation/messages.go` | Unexported message types and their `handle` methods. |
| `harness/inspected/simulation/traffic.go` | Unexported seeded traffic generator. |
| `harness/inspected/simulation/simulation_test.go` | Actor tests. |
| `.go-arch-lint.yml` | Component `simulation`, rule `mayDependOn: [fulfillment]`. |

## Implementation Tasks

1. Write `simulation_test.go` with every test listed below, including the `recorderStub` helper. Confirm it fails to compile.
2. Create `simulation.go` with the public API.
3. Create `messages.go` with one message type per request.
4. Create `traffic.go`.
5. Create `doc.go`.
6. In `.go-arch-lint.yml` add component `simulation: { in: harness/inspected/simulation }` and under `deps:` add `simulation: { mayDependOn: [fulfillment] }`.
7. Run `go test ./harness/inspected/simulation/...` until it passes, then `task all`.

## Technical Details

### Public API (`simulation.go`)

```go
const (
    MaxOrdersPerTick = 100  // upper bound of Config.OrdersPerTick
    MaxAdvanceTicks  = 1000 // upper bound of one Advance request
)

var (
    ErrStopped        = errors.New("simulation stopped")
    ErrAlreadyRunning = errors.New("simulation already running")
    ErrInvalidConfig  = errors.New("invalid simulation config")
    ErrInvalidAdvance = errors.New("invalid advance")
)

// Config is validated by New. Every field is required and has no default.
type Config struct {
    Seed          uint64 // seed of the traffic generator
    OrdersPerTick int    // simulated orders placed after every tick, 0..MaxOrdersPerTick
}

func (c Config) Validate() error // wraps ErrInvalidConfig

// Recorder receives every event and the state after every mutation.
// It is called only from the Run goroutine and must not block.
type Recorder interface {
    Record(events []fulfillment.Event)
    Observe(s fulfillment.Snapshot)
}

type Status struct {
    Now          fulfillment.Tick
    ClockRunning bool
    Dependencies []fulfillment.Dependency
}

type Simulation struct {
    // Owned by the Run goroutine. Never read or written elsewhere.
    state        *fulfillment.State
    traffic      *traffic
    clockRunning bool
    recorder     Recorder

    // Immutable after New.
    mailbox chan message  // unbuffered
    done    chan struct{} // closed when Run returns
    running atomic.Bool   // guards against a second Run
}

func New(cfg Config, catalog []fulfillment.Product, rec Recorder) (*Simulation, error)
func (s *Simulation) Run(ctx context.Context, clock <-chan time.Time) error

func (s *Simulation) PlaceOrder(ctx context.Context, req fulfillment.OrderRequest) (fulfillment.Order, error)
func (s *Simulation) Snapshot(ctx context.Context) (fulfillment.Snapshot, error)
func (s *Simulation) Status(ctx context.Context) (Status, error)
func (s *Simulation) Advance(ctx context.Context, ticks int) (Status, error)
func (s *Simulation) SetClockRunning(ctx context.Context, running bool) (Status, error)
func (s *Simulation) SetDependencyMode(ctx context.Context, name fulfillment.DependencyName, mode fulfillment.DependencyMode) (Status, error)
```

`New` validates `cfg`, calls `fulfillment.NewState(catalog)`, builds the traffic generator from the sorted catalog SKUs, sets `clockRunning = true` and returns an error when `rec` is nil. `New` starts no goroutine.

### Lifecycle (`Run`)

```go
func (s *Simulation) Run(ctx context.Context, clock <-chan time.Time) error {
    if !s.running.CompareAndSwap(false, true) {
        return ErrAlreadyRunning
    }
    defer close(s.done)
    s.recorder.Observe(s.state.Snapshot())
    for {
        select {
        case <-ctx.Done():
            return nil
        case msg := <-s.mailbox:
            if err := msg.handle(s); err != nil {
                return fmt.Errorf("simulation: %w", err)
            }
        case <-clock:
            if s.clockRunning {
                if err := s.advance(1); err != nil {
                    return fmt.Errorf("simulation: clock tick: %w", err)
                }
            }
        }
    }
}
```

- `Run` returns nil when `ctx` is cancelled. It returns a non-nil error only for a system failure (a generated order rejected by the domain, which is a bug). The caller (step 05 `App.Run`, step 07 `workbench.Run`) is the supervisor.
- `clock` may be nil. A nil channel never fires, so the simulation advances only through `Advance`.
- After `Run` returns, every request method returns an error wrapping `ErrStopped`.

### Advancing time

```go
// advance runs n ticks. Called only from the Run goroutine.
func (s *Simulation) advance(n int) error {
    for range n {
        events := s.state.Tick()
        for range s.traffic.ordersPerTick {
            _, placed, err := s.state.PlaceOrder(s.traffic.next(), fulfillment.ChannelSimulation)
            if err != nil {
                return fmt.Errorf("generate order: %w", err)
            }
            events = append(events, placed...)
        }
        s.recorder.Record(events)
    }
    s.recorder.Observe(s.state.Snapshot())
    return nil
}
```

Per tick the order is: domain tick first, then simulated orders placed at the new tick.

### Messages (`messages.go`)

```go
// message is a request processed by the Run goroutine.
// handle applies it to the owned state and sends exactly one reply.
// A returned error is a system failure and stops Run.
type message interface {
    handle(s *Simulation) error
}
```

| Type | Fields | Handling | Reply |
| --- | --- | --- | --- |
| `placeOrder` | `req fulfillment.OrderRequest`, `reply chan orderReply` | `state.PlaceOrder(req, ChannelAPI)`; on success `Record`, `Observe` | `orderReply{order, err}` |
| `readSnapshot` | `reply chan fulfillment.Snapshot` | `state.Snapshot()` | snapshot |
| `readStatus` | `reply chan Status` | build `Status` | status |
| `advanceTime` | `ticks int`, `reply chan statusReply` | validate `1 <= ticks <= MaxAdvanceTicks` (else reply error wrapping `ErrInvalidAdvance`), `advance(ticks)` | `statusReply{status, err}` |
| `setClock` | `running bool`, `reply chan Status` | set `clockRunning` | status |
| `setDependencyMode` | `name`, `mode`, `reply chan statusReply` | `state.SetDependencyMode`; on success `Observe` | `statusReply{status, err}` |

Every reply channel is created with capacity 1, so `handle` never blocks when the caller is gone. Business errors (`ErrInvalidAdvance`, domain validation errors) travel in the reply. Only `advance` errors are returned from `handle`.

### Request helper

```go
// call sends msg and waits for the reply. It honors ctx and actor shutdown.
func call[T any](ctx context.Context, s *Simulation, msg message, reply <-chan T) (T, error)
```

1. `select` on `s.mailbox <- msg`, `<-ctx.Done()` (return `ctx.Err()`), `<-s.done` (return `ErrStopped`).
2. `select` on `<-reply`, `<-ctx.Done()`, `<-s.done`.

Delivery is at most once. When `ctx` ends after the message was accepted, the mutation may already be applied. This is documented in `doc.go`.

### Traffic (`traffic.go`)

```go
const maxGeneratedQuantity = 5

type traffic struct {
    rng           *rand.Rand // math/rand/v2
    ordersPerTick int
    skus          []fulfillment.SKU // catalog SKUs in sorted order
}

func newTraffic(seed uint64, ordersPerTick int, skus []fulfillment.SKU) *traffic {
    return &traffic{rng: rand.New(rand.NewPCG(seed, seed)), ordersPerTick: ordersPerTick, skus: skus}
}

func (t *traffic) next() fulfillment.OrderRequest {
    return fulfillment.OrderRequest{
        SKU:      t.skus[t.rng.IntN(len(t.skus))],
        Quantity: 1 + t.rng.IntN(maxGeneratedQuantity),
    }
}
```

### Determinism and restart

For the same `Config`, catalog and message sequence, the snapshot sequence is identical. State is in memory only; a restart is a new `Simulation` from the same `Config` and starts at tick 0. This is the documented restart behavior.

### Tests (`simulation_test.go`)

Helper types and functions:

```go
// recorderStub collects calls. Read it only after a reply was received;
// the reply channel establishes happens-before with the Run goroutine.
type recorderStub struct {
    events    []fulfillment.Event
    snapshots []fulfillment.Snapshot
}

// start runs sim.Run with the given clock and stops it on test cleanup,
// requiring Run to return nil.
func start(t *testing.T, sim *simulation.Simulation, clock <-chan time.Time)
```

Default test config: `Config{Seed: 1, OrdersPerTick: 0}`, catalog `fulfillment.StandardCatalog()`, clock nil.

| Test | Assertion |
| --- | --- |
| `TestNewRejectsInvalidConfig` | Table: `OrdersPerTick` -1 and 101 return `ErrInvalidConfig`; nil recorder returns error; empty catalog returns `fulfillment.ErrInvalidCatalog`. |
| `TestRunObservesInitialState` | After the first `Status` reply, `recorderStub.snapshots[0].Now == 0` and it has 5 products. |
| `TestPlaceOrder` | Returns `ord-000001` `pending` with channel `api`; `Snapshot` contains it; recorder got `OrderPlaced`. |
| `TestPlaceOrderRejectsInvalidRequest` | Quantity 0 returns `fulfillment.ErrInvalidQuantity`; `Run` keeps running (a following `Status` succeeds). |
| `TestAdvance` | Place `sku-001` x1, `Advance(ctx, 2)` returns `Status.Now == 2`; order is `shipped`. |
| `TestAdvanceRejectsInvalidTicks` | Table 0, -1, 1001: error wraps `ErrInvalidAdvance`; `Status.Now == 0`. |
| `TestClockTickAdvancesWhenRunning` | Clock is a test-owned unbuffered `chan time.Time`; send one value; `Status.Now == 1`. |
| `TestPausedClockIgnoresTicks` | `SetClockRunning(false)`; send one clock value; `Now == 0`; `Advance(1)`; `Now == 1`; `SetClockRunning(true)`, send one value; `Now == 2`. |
| `TestSetDependencyMode` | `payment-gateway` to `outage` returns status with that mode; last recorded snapshot shows it; unknown name returns `fulfillment.ErrUnknownDependency`. |
| `TestTrafficIsDeterministicForSeed` | Two simulations, `Seed 42`, `OrdersPerTick 2`, `Advance(20)` each: snapshots `require.Equal`; `len(snapshot.Orders) == 40`. |
| `TestTrafficDiffersForOtherSeed` | Seeds 42 and 43, same advance: `Orders` slices not equal. |
| `TestRestartReplaysFromSeed` | Sim A: `Advance(10)`, snapshot, cancel. Calls on A return `ErrStopped`. Sim B with same config: `Advance(10)`, snapshot equals A's. |
| `TestRunReturnsNilOnCancel` | Cancel ctx; `Run` returns nil. |
| `TestRunTwiceFails` | Second `Run` on a running simulation returns `ErrAlreadyRunning` immediately. |
| `TestRequestWithCancelledContext` | Simulation never started; `Status` with an already cancelled ctx returns `context.Canceled`. |
| `TestRecorderReceivesEventsInOrder` | Place, `Advance(2)`: recorded event types in order are `OrderPlaced`, `DependencyCalled`, `OrderStatusChanged`, `DependencyCalled`, `OrderStatusChanged`. |

Actor tests use plain Go and `require`. The `ergo.services/ergo/testing` packages do not apply because the actor is not an ergo process.

## Verification

```powershell
go test ./harness/inspected/simulation/...
grep -rnE "time\.Sleep|time\.After" harness/inspected/simulation
grep -rnE "sync\.(RW)?Mutex" harness/inspected/simulation
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/inspected/simulation/...` exits 0 and runs every test listed above.
- `grep -rnE "time\.Sleep|time\.After" harness/inspected/simulation` prints nothing.
- `grep -rnE "sync\.(RW)?Mutex" harness/inspected/simulation` prints nothing.
- `harness/inspected/simulation` imports no module outside stdlib and `harness/inspected/fulfillment`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Creating the wall-clock ticker. The caller passes the clock channel (step 05).
- Metrics or HTTP code.
- Persistence or state recovery after restart.
- Supervision and restart. The caller owns it.
