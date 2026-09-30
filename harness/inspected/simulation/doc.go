// Package simulation runs the fulfillment domain as an actor.
//
// # Ownership
//
// A Simulation owns one fulfillment.State. Only the goroutine executing Run
// reads or writes it. Every other goroutine talks to it through messages sent
// over an unbuffered mailbox, and every message gets exactly one reply on a
// channel of capacity 1. State is never shared and there are no locks.
//
// # Messages
//
// The exported request methods are the message contracts:
//
//   - PlaceOrder places an order on the api channel.
//   - Snapshot and Status read state.
//   - Advance runs 1..MaxAdvanceTicks ticks.
//   - SetClockRunning pauses or resumes the clock.
//   - SetDependencyMode changes a dependency mode.
//
// Business failures (invalid request, unknown dependency, invalid advance)
// are returned to the caller and do not affect the actor. Requests honor the
// caller's context. Delivery is at most once: when the context ends after the
// actor accepted the message, the request may still have been applied.
//
// # Lifecycle
//
// New creates a Simulation and starts nothing. Run starts the actor loop and
// blocks until its context is cancelled, then returns nil. Run returns a
// non-nil error only for a system failure, meaning a bug: the traffic
// generator produced an order the domain rejected. After Run returns, every
// request returns ErrStopped. Run may be called once.
//
// # Time
//
// Time moves in two ways: a value received from the clock channel passed to
// Run advances one tick while the clock is running, and Advance runs ticks on
// demand, also while the clock is paused. The caller owns the clock; a nil
// clock never fires. Per tick, the domain ticks first, then the traffic
// generator places OrdersPerTick orders at the new tick.
//
// # Determinism
//
// For the same Config, catalog and message sequence the states are identical.
// The traffic generator is a seeded PCG source owned by the actor.
//
// # Failure boundary and restart
//
// The caller is the supervisor. State lives in memory only, so a restart is a
// new Simulation created from the same Config. It starts at tick 0 and
// replays the same generated traffic; orders placed through PlaceOrder and
// dependency modes set before the restart are lost.
//
// # Observation
//
// The Recorder receives every event and a snapshot after every mutation. It
// runs on the actor goroutine and must not block.
package simulation
