package simulation_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/simulation"
)

// recorderStub collects calls. Read it only after a reply was received from
// the simulation: the reply establishes happens-before with the Run goroutine.
type recorderStub struct {
	events    []fulfillment.Event
	snapshots []fulfillment.Snapshot
}

func (r *recorderStub) Record(events []fulfillment.Event) { r.events = append(r.events, events...) }
func (r *recorderStub) Observe(s fulfillment.Snapshot)    { r.snapshots = append(r.snapshots, s) }

// running is a started Run goroutine.
type running struct {
	cancel context.CancelFunc
	done   chan error
	once   sync.Once
	err    error
}

// stop cancels Run and returns its result. It is safe to call repeatedly.
func (r *running) stop() error {
	r.once.Do(func() {
		r.cancel()
		r.err = <-r.done
	})
	return r.err
}

// start runs sim.Run with the given clock and, on test cleanup, stops it and
// requires Run to return nil.
func start(t *testing.T, sim *simulation.Simulation, clock <-chan time.Time) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- sim.Run(ctx, clock) }()
	t.Cleanup(func() { require.NoError(t, r.stop()) })
	return r
}

func newSim(t *testing.T, cfg simulation.Config) (*simulation.Simulation, *recorderStub) {
	t.Helper()
	rec := &recorderStub{}
	sim, err := simulation.New(cfg, fulfillment.StandardCatalog(), rec)
	require.NoError(t, err)
	return sim, rec
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	for _, n := range []int{-1, simulation.MaxOrdersPerTick + 1} {
		_, err := simulation.New(simulation.Config{OrdersPerTick: n}, fulfillment.StandardCatalog(), &recorderStub{})
		require.ErrorIs(t, err, simulation.ErrInvalidConfig, n)
	}

	_, err := simulation.New(simulation.Config{}, fulfillment.StandardCatalog(), nil)
	require.ErrorIs(t, err, simulation.ErrInvalidConfig)

	_, err = simulation.New(simulation.Config{}, nil, &recorderStub{})
	require.ErrorIs(t, err, fulfillment.ErrInvalidCatalog)
}

func TestRunObservesInitialState(t *testing.T) {
	sim, rec := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)

	_, err := sim.Status(context.Background())
	require.NoError(t, err)

	require.NotEmpty(t, rec.snapshots)
	require.Equal(t, fulfillment.Tick(0), rec.snapshots[0].Now)
	require.Len(t, rec.snapshots[0].Products, 5)
}

func TestPlaceOrder(t *testing.T) {
	sim, rec := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	o, err := sim.PlaceOrder(ctx, fulfillment.OrderRequest{SKU: "sku-001", Quantity: 1})
	require.NoError(t, err)
	require.Equal(t, fulfillment.OrderID("ord-000001"), o.ID)
	require.Equal(t, fulfillment.StatusPending, o.Status)
	require.Equal(t, fulfillment.ChannelAPI, o.Channel)

	snap, err := sim.Snapshot(ctx)
	require.NoError(t, err)
	_, ok := snap.Order(o.ID)
	require.True(t, ok)
	require.Equal(t, []fulfillment.Event{
		fulfillment.OrderPlaced{Order: o.ID, SKU: "sku-001", Quantity: 1, Channel: fulfillment.ChannelAPI, At: 0},
	}, rec.events)
}

func TestPlaceOrderRejectsInvalidRequest(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	_, err := sim.PlaceOrder(ctx, fulfillment.OrderRequest{SKU: "sku-001", Quantity: 0})
	require.ErrorIs(t, err, fulfillment.ErrInvalidQuantity)

	_, err = sim.Status(ctx) // the actor is still running
	require.NoError(t, err)
}

func TestAdvance(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	o, err := sim.PlaceOrder(ctx, fulfillment.OrderRequest{SKU: "sku-001", Quantity: 1})
	require.NoError(t, err)

	status, err := sim.Advance(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(2), status.Now)

	snap, err := sim.Snapshot(ctx)
	require.NoError(t, err)
	got, _ := snap.Order(o.ID)
	require.Equal(t, fulfillment.StatusShipped, got.Status)
}

func TestAdvanceRejectsInvalidTicks(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	for _, n := range []int{0, -1, simulation.MaxAdvanceTicks + 1} {
		_, err := sim.Advance(ctx, n)
		require.ErrorIs(t, err, simulation.ErrInvalidAdvance, n)
	}

	status, err := sim.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(0), status.Now)
}

func TestClockTickAdvancesWhenRunning(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	clock := make(chan time.Time)
	start(t, sim, clock)

	clock <- time.Time{}

	status, err := sim.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(1), status.Now)
}

func TestPausedClockIgnoresTicks(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	clock := make(chan time.Time)
	start(t, sim, clock)
	ctx := context.Background()

	status, err := sim.SetClockRunning(ctx, false)
	require.NoError(t, err)
	require.False(t, status.ClockRunning)

	clock <- time.Time{}
	status, err = sim.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(0), status.Now)

	status, err = sim.Advance(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(1), status.Now)

	_, err = sim.SetClockRunning(ctx, true)
	require.NoError(t, err)
	clock <- time.Time{}
	status, err = sim.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Tick(2), status.Now)
}

func TestSetDependencyMode(t *testing.T) {
	sim, rec := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	status, err := sim.SetDependencyMode(ctx, fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage)
	require.NoError(t, err)
	require.Equal(t, fulfillment.Dependency{Name: fulfillment.DependencyPaymentGateway, Mode: fulfillment.ModeOutage}, status.Dependencies[0])
	require.Equal(t, fulfillment.ModeOutage, rec.snapshots[len(rec.snapshots)-1].Dependencies[0].Mode)

	_, err = sim.SetDependencyMode(ctx, "unknown", fulfillment.ModeSlow)
	require.ErrorIs(t, err, fulfillment.ErrUnknownDependency)
}

func advanced(t *testing.T, seed uint64, ticks int) fulfillment.Snapshot {
	t.Helper()
	sim, _ := newSim(t, simulation.Config{Seed: seed, OrdersPerTick: 2})
	start(t, sim, nil)
	_, err := sim.Advance(context.Background(), ticks)
	require.NoError(t, err)
	snap, err := sim.Snapshot(context.Background())
	require.NoError(t, err)
	return snap
}

func TestTrafficIsDeterministicForSeed(t *testing.T) {
	a := advanced(t, 42, 20)
	b := advanced(t, 42, 20)

	require.Equal(t, a, b)
	require.Len(t, a.Orders, 40)
}

func TestTrafficDiffersForOtherSeed(t *testing.T) {
	require.NotEqual(t, advanced(t, 42, 20).Orders, advanced(t, 43, 20).Orders)
}

func TestRestartReplaysFromSeed(t *testing.T) {
	cfg := simulation.Config{Seed: 42, OrdersPerTick: 2}
	ctx := context.Background()

	a, _ := newSim(t, cfg)
	runA := start(t, a, nil)
	_, err := a.Advance(ctx, 10)
	require.NoError(t, err)
	snapA, err := a.Snapshot(ctx)
	require.NoError(t, err)
	require.NoError(t, runA.stop())

	_, err = a.Snapshot(ctx)
	require.ErrorIs(t, err, simulation.ErrStopped)

	b, _ := newSim(t, cfg)
	start(t, b, nil)
	_, err = b.Advance(ctx, 10)
	require.NoError(t, err)
	snapB, err := b.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, snapA, snapB)
}

func TestRunReturnsNilOnCancel(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	r := start(t, sim, nil)

	require.NoError(t, r.stop())
}

func TestRunTwiceFails(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	_, err := sim.Status(context.Background()) // proves the first Run started
	require.NoError(t, err)

	err = sim.Run(context.Background(), nil)

	require.ErrorIs(t, err, simulation.ErrAlreadyRunning)
}

func TestRequestWithCancelledContext(t *testing.T) {
	sim, _ := newSim(t, simulation.Config{Seed: 1}) // never started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sim.Status(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func TestRecorderReceivesEventsInOrder(t *testing.T) {
	sim, rec := newSim(t, simulation.Config{Seed: 1})
	start(t, sim, nil)
	ctx := context.Background()

	_, err := sim.PlaceOrder(ctx, fulfillment.OrderRequest{SKU: "sku-001", Quantity: 1})
	require.NoError(t, err)
	_, err = sim.Advance(ctx, 2)
	require.NoError(t, err)

	types := make([]string, 0, len(rec.events))
	for _, e := range rec.events {
		switch e.(type) {
		case fulfillment.OrderPlaced:
			types = append(types, "OrderPlaced")
		case fulfillment.DependencyCalled:
			types = append(types, "DependencyCalled")
		case fulfillment.OrderStatusChanged:
			types = append(types, "OrderStatusChanged")
		default:
			t.Fatalf("unexpected event %T", e)
		}
	}
	require.Equal(t, []string{
		"OrderPlaced", "DependencyCalled", "OrderStatusChanged", "DependencyCalled", "OrderStatusChanged",
	}, types)
}
