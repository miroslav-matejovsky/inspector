package simulation

import (
	"fmt"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

// message is a request processed by the Run goroutine. handle applies it to
// the owned state and sends exactly one reply. A returned error is a system
// failure and stops Run; business errors travel in the reply.
//
// Every reply channel has capacity 1, so handle never blocks when the caller
// has given up.
type message interface {
	handle(s *Simulation) error
}

type orderReply struct {
	order fulfillment.Order
	err   error
}

type statusReply struct {
	status Status
	err    error
}

type placeOrder struct {
	req   fulfillment.OrderRequest
	reply chan orderReply
}

func (m placeOrder) handle(s *Simulation) error {
	order, events, err := s.state.PlaceOrder(m.req, fulfillment.ChannelAPI)
	if err == nil {
		s.recorder.Record(events)
		s.recorder.Observe(s.state.Snapshot())
	}
	m.reply <- orderReply{order: order, err: err}
	return nil
}

type readSnapshot struct {
	reply chan fulfillment.Snapshot
}

func (m readSnapshot) handle(s *Simulation) error {
	m.reply <- s.state.Snapshot()
	return nil
}

type readStatus struct {
	reply chan Status
}

func (m readStatus) handle(s *Simulation) error {
	m.reply <- s.status()
	return nil
}

type advanceTime struct {
	ticks int
	reply chan statusReply
}

func (m advanceTime) handle(s *Simulation) error {
	if m.ticks < 1 || m.ticks > MaxAdvanceTicks {
		m.reply <- statusReply{err: fmt.Errorf("%w: %d ticks, want 1..%d", ErrInvalidAdvance, m.ticks, MaxAdvanceTicks)}
		return nil
	}
	if err := s.advance(m.ticks); err != nil {
		m.reply <- statusReply{err: err}
		return err
	}
	m.reply <- statusReply{status: s.status()}
	return nil
}

type setClock struct {
	running bool
	reply   chan Status
}

func (m setClock) handle(s *Simulation) error {
	s.clockRunning = m.running
	m.reply <- s.status()
	return nil
}

type setDependencyMode struct {
	name  fulfillment.DependencyName
	mode  fulfillment.DependencyMode
	reply chan statusReply
}

func (m setDependencyMode) handle(s *Simulation) error {
	err := s.state.SetDependencyMode(m.name, m.mode)
	if err != nil {
		m.reply <- statusReply{err: err}
		return nil
	}
	s.recorder.Observe(s.state.Snapshot())
	m.reply <- statusReply{status: s.status()}
	return nil
}
