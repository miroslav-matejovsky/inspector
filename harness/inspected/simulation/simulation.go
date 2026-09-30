package simulation

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

const (
	// MaxOrdersPerTick is the upper bound of Config.OrdersPerTick.
	MaxOrdersPerTick = 100
	// MaxAdvanceTicks is the upper bound of one Advance request.
	MaxAdvanceTicks = 1000
)

// Errors of the simulation. Returned errors wrap these with context.
var (
	// ErrStopped is returned by requests after Run has returned.
	ErrStopped = errors.New("simulation stopped")
	// ErrAlreadyRunning is returned by a second call to Run.
	ErrAlreadyRunning = errors.New("simulation already running")
	// ErrInvalidConfig is returned by New for an invalid Config or Recorder.
	ErrInvalidConfig = errors.New("invalid simulation config")
	// ErrInvalidAdvance is returned by Advance for a tick count out of range.
	ErrInvalidAdvance = errors.New("invalid advance")
)

// Config configures a Simulation. Every field is required and has no default.
type Config struct {
	Seed          uint64 // seed of the traffic generator
	OrdersPerTick int    // simulated orders placed after every tick, 0..MaxOrdersPerTick
}

// Validate reports whether the config is usable. The error wraps ErrInvalidConfig.
func (c Config) Validate() error {
	if c.OrdersPerTick < 0 || c.OrdersPerTick > MaxOrdersPerTick {
		return fmt.Errorf("%w: orders per tick %d, want 0..%d", ErrInvalidConfig, c.OrdersPerTick, MaxOrdersPerTick)
	}
	return nil
}

// Recorder receives every event and the state after every mutation. It is
// called only from the Run goroutine and must not block.
type Recorder interface {
	Record(events []fulfillment.Event)
	Observe(s fulfillment.Snapshot)
}

// Status is the control view of a Simulation.
type Status struct {
	Now          fulfillment.Tick
	ClockRunning bool
	Dependencies []fulfillment.Dependency
}

// Simulation owns a fulfillment.State and is its only writer. See the package
// documentation for the message contract and lifecycle.
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

// New validates cfg and creates a Simulation at tick 0 with the clock running.
// It starts no goroutine; call Run.
func New(cfg Config, catalog []fulfillment.Product, rec Recorder) (*Simulation, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("%w: recorder is required", ErrInvalidConfig)
	}
	state, err := fulfillment.NewState(catalog)
	if err != nil {
		return nil, fmt.Errorf("simulation: %w", err)
	}

	products := state.Snapshot().Products
	skus := make([]fulfillment.SKU, 0, len(products))
	for _, p := range products {
		skus = append(skus, p.SKU)
	}

	return &Simulation{
		state:        state,
		traffic:      newTraffic(cfg.Seed, cfg.OrdersPerTick, skus),
		clockRunning: true,
		recorder:     rec,
		mailbox:      make(chan message),
		done:         make(chan struct{}),
	}, nil
}

// Run processes requests and clock ticks until ctx is cancelled. It returns
// nil on cancellation and a non-nil error only for a system failure, which is
// a bug. A nil clock never fires, so time then moves only through Advance.
// Run may be called once.
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

// advance runs n ticks. Per tick the domain ticks first, then the traffic
// generator places its orders at the new tick. Called only from Run.
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

func (s *Simulation) status() Status {
	snap := s.state.Snapshot()
	return Status{Now: snap.Now, ClockRunning: s.clockRunning, Dependencies: snap.Dependencies}
}

// call sends msg and waits for the reply. It honors ctx and actor shutdown.
// Delivery is at most once: when ctx ends after the actor accepted msg, the
// request may still have been applied.
func call[T any](ctx context.Context, s *Simulation, msg message, reply <-chan T) (T, error) {
	var zero T
	select {
	case s.mailbox <- msg:
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-s.done:
		return zero, ErrStopped
	}
	select {
	case r := <-reply:
		return r, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-s.done:
		return zero, ErrStopped
	}
}

// PlaceOrder places an order on the api channel. Domain validation errors
// wrap the fulfillment sentinel errors.
func (s *Simulation) PlaceOrder(ctx context.Context, req fulfillment.OrderRequest) (fulfillment.Order, error) {
	reply := make(chan orderReply, 1)
	r, err := call(ctx, s, placeOrder{req: req, reply: reply}, reply)
	if err != nil {
		return fulfillment.Order{}, err
	}
	return r.order, r.err
}

// Snapshot returns a copy of the current state.
func (s *Simulation) Snapshot(ctx context.Context) (fulfillment.Snapshot, error) {
	reply := make(chan fulfillment.Snapshot, 1)
	return call(ctx, s, readSnapshot{reply: reply}, reply)
}

// Status returns the current tick, clock state and dependency modes.
func (s *Simulation) Status(ctx context.Context) (Status, error) {
	reply := make(chan Status, 1)
	return call(ctx, s, readStatus{reply: reply}, reply)
}

// Advance runs ticks ticks regardless of the clock state. ticks must be in
// 1..MaxAdvanceTicks, otherwise the error wraps ErrInvalidAdvance.
func (s *Simulation) Advance(ctx context.Context, ticks int) (Status, error) {
	reply := make(chan statusReply, 1)
	r, err := call(ctx, s, advanceTime{ticks: ticks, reply: reply}, reply)
	if err != nil {
		return Status{}, err
	}
	return r.status, r.err
}

// SetClockRunning pauses or resumes the clock. Advance works while paused.
func (s *Simulation) SetClockRunning(ctx context.Context, running bool) (Status, error) {
	reply := make(chan Status, 1)
	return call(ctx, s, setClock{running: running, reply: reply}, reply)
}

// SetDependencyMode changes the mode of a dependency. Errors wrap
// fulfillment.ErrUnknownDependency and fulfillment.ErrInvalidDependencyMode.
func (s *Simulation) SetDependencyMode(ctx context.Context, name fulfillment.DependencyName, mode fulfillment.DependencyMode) (Status, error) {
	reply := make(chan statusReply, 1)
	r, err := call(ctx, s, setDependencyMode{name: name, mode: mode, reply: reply}, reply)
	if err != nil {
		return Status{}, err
	}
	return r.status, r.err
}
