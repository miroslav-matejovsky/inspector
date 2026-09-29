package fulfillment

import (
	"fmt"
	"slices"
	"strings"
)

// Tick is logical simulation time. A new State is at tick 0.
type Tick uint64

// Business rule constants. See the package documentation.
const (
	// PaymentLimitCents: payment is declined when TotalCents exceeds it.
	PaymentLimitCents = 50000
	// MaxOrderQuantity: a valid quantity is 1..MaxOrderQuantity.
	MaxOrderQuantity = 10
	// HealthyLatencyTicks: ticks a stage waits before calling a healthy or
	// failing dependency.
	HealthyLatencyTicks = 1
	// SlowLatencyTicks: ticks a stage waits before calling a slow dependency.
	SlowLatencyTicks = 3
	// MaxStageAttempts: failed dependency calls per stage before the order fails.
	MaxStageAttempts = 3
	// RestockIntervalTicks: a restock runs when the tick is a multiple of it.
	RestockIntervalTicks = 10
	// MaxRetainedOrders: at the end of every tick, the oldest terminal orders
	// above this count are dropped. Orders that are not terminal are never dropped.
	MaxRetainedOrders = 1000
)

// State is the complete state of the fulfillment service. All rules are
// deterministic: the same call sequence always produces the same states and
// events. State has no synchronization; it must be owned by one goroutine.
type State struct {
	now          Tick
	products     []Product // sorted by SKU
	orders       []*orderRecord
	nextOrder    uint64
	dependencies []Dependency // sorted by name
}

// orderRecord is an order plus the bookkeeping of its current stage.
type orderRecord struct {
	order     Order
	waitTicks int // ticks left before the current stage calls its dependency
	attempts  int // failed dependency calls in the current stage
}

// NewState returns a State at tick 0 with both dependencies healthy. The
// catalog is copied. An invalid catalog returns an error wrapping
// ErrInvalidCatalog.
func NewState(catalog []Product) (*State, error) {
	if len(catalog) == 0 {
		return nil, fmt.Errorf("%w: empty", ErrInvalidCatalog)
	}
	products := slices.Clone(catalog)
	slices.SortFunc(products, func(a, b Product) int { return strings.Compare(string(a.SKU), string(b.SKU)) })
	for i, p := range products {
		if err := validateProduct(p); err != nil {
			return nil, fmt.Errorf("%w: product %q: %s", ErrInvalidCatalog, p.SKU, err)
		}
		if i > 0 && products[i-1].SKU == p.SKU {
			return nil, fmt.Errorf("%w: duplicate sku %q", ErrInvalidCatalog, p.SKU)
		}
	}

	deps := make([]Dependency, 0, len(DependencyNames()))
	for _, name := range DependencyNames() {
		deps = append(deps, Dependency{Name: name, Mode: ModeHealthy})
	}
	return &State{products: products, dependencies: deps}, nil
}

func validateProduct(p Product) error {
	switch {
	case p.SKU == "":
		return fmt.Errorf("empty sku")
	case p.Name == "":
		return fmt.Errorf("empty name")
	case p.PriceCents <= 0:
		return fmt.Errorf("price must be positive")
	case p.Capacity <= 0:
		return fmt.Errorf("capacity must be positive")
	case p.Stock < 0 || p.Stock > p.Capacity:
		return fmt.Errorf("stock %d outside 0..%d", p.Stock, p.Capacity)
	case p.ReorderPoint < 0 || p.ReorderPoint >= p.Capacity:
		return fmt.Errorf("reorder point %d outside 0..%d", p.ReorderPoint, p.Capacity-1)
	}
	return nil
}

// PlaceOrder accepts an order in status pending. Stock is not checked or
// reserved here. On error the state is unchanged. It returns a copy of the
// order and the events the placement produced.
func (s *State) PlaceOrder(req OrderRequest, channel Channel) (Order, []Event, error) {
	if !slices.Contains(Channels(), channel) {
		return Order{}, nil, fmt.Errorf("%w: %q", ErrInvalidChannel, channel)
	}
	product := s.product(req.SKU)
	if product == nil {
		return Order{}, nil, fmt.Errorf("%w: %q", ErrUnknownProduct, req.SKU)
	}
	if req.Quantity < 1 || req.Quantity > MaxOrderQuantity {
		return Order{}, nil, fmt.Errorf("%w: %d, want 1..%d", ErrInvalidQuantity, req.Quantity, MaxOrderQuantity)
	}

	s.nextOrder++
	rec := &orderRecord{
		order: Order{
			ID:         OrderID(fmt.Sprintf("ord-%06d", s.nextOrder)),
			SKU:        req.SKU,
			Quantity:   req.Quantity,
			TotalCents: product.PriceCents * int64(req.Quantity),
			Channel:    channel,
			Status:     StatusPending,
			PlacedAt:   s.now,
			UpdatedAt:  s.now,
			History:    []Transition{{To: StatusPending, At: s.now, Reason: ReasonOrderPlaced}},
		},
		waitTicks: latency(s.mode(DependencyPaymentGateway)),
	}
	s.orders = append(s.orders, rec)

	placed := OrderPlaced{Order: rec.order.ID, SKU: req.SKU, Quantity: req.Quantity, Channel: channel, At: s.now}
	return rec.order.clone(), []Event{placed}, nil
}

// Tick advances logical time by one tick and returns the events produced, in
// the order they happened. Orders are processed in placement order.
func (s *State) Tick() []Event {
	s.now++
	var events []Event
	for _, rec := range s.orders {
		if rec.order.Status.Terminal() {
			continue
		}
		rec.waitTicks--
		if rec.waitTicks > 0 {
			continue
		}
		switch rec.order.Status {
		case StatusPending:
			events = s.processPayment(rec, events)
		case StatusPaid:
			events = s.processShipment(rec, events)
		}
	}
	if s.now%RestockIntervalTicks == 0 && s.mode(DependencyWarehouse) != ModeOutage {
		events = s.restock(events)
	}
	s.prune()
	return events
}

func (s *State) processPayment(rec *orderRecord, events []Event) []Event {
	if s.mode(DependencyPaymentGateway) == ModeOutage {
		events = append(events, DependencyCalled{Dependency: DependencyPaymentGateway, Succeeded: false, At: s.now})
		rec.attempts++
		if rec.attempts >= MaxStageAttempts {
			return s.transition(rec, StatusFailed, ReasonPaymentGatewayUnavailable, events)
		}
		rec.waitTicks = HealthyLatencyTicks
		return events
	}
	events = append(events, DependencyCalled{Dependency: DependencyPaymentGateway, Succeeded: true, At: s.now})
	if rec.order.TotalCents > PaymentLimitCents {
		return s.transition(rec, StatusFailed, ReasonPaymentDeclined, events)
	}
	rec.attempts = 0
	rec.waitTicks = latency(s.mode(DependencyWarehouse))
	return s.transition(rec, StatusPaid, ReasonPaymentAuthorized, events)
}

func (s *State) processShipment(rec *orderRecord, events []Event) []Event {
	if s.mode(DependencyWarehouse) == ModeOutage {
		events = append(events, DependencyCalled{Dependency: DependencyWarehouse, Succeeded: false, At: s.now})
		rec.attempts++
		if rec.attempts >= MaxStageAttempts {
			return s.transition(rec, StatusFailed, ReasonWarehouseUnavailable, events)
		}
		rec.waitTicks = HealthyLatencyTicks
		return events
	}
	events = append(events, DependencyCalled{Dependency: DependencyWarehouse, Succeeded: true, At: s.now})
	product := s.product(rec.order.SKU)
	if product.Stock < rec.order.Quantity {
		return s.transition(rec, StatusFailed, ReasonOutOfStock, events)
	}
	product.Stock -= rec.order.Quantity
	return s.transition(rec, StatusShipped, ReasonShipped, events)
}

// transition moves an order to a new status. A change that is not in
// AllowedTransitions is a programming error and panics.
func (s *State) transition(rec *orderRecord, to OrderStatus, reason Reason, events []Event) []Event {
	from := rec.order.Status
	rule := TransitionRule{From: from, To: to, Reason: reason}
	if !slices.Contains(AllowedTransitions(), rule) {
		panic(fmt.Sprintf("fulfillment: transition %s -> %s (%s) is not allowed", from, to, reason))
	}
	rec.order.Status = to
	rec.order.UpdatedAt = s.now
	if to == StatusFailed {
		rec.order.FailureReason = reason
	}
	rec.order.History = append(rec.order.History, Transition{From: from, To: to, At: s.now, Reason: reason})
	return append(events, OrderStatusChanged{
		Order: rec.order.ID, SKU: rec.order.SKU, From: from, To: to,
		Reason: reason, PlacedAt: rec.order.PlacedAt, At: s.now,
	})
}

func (s *State) restock(events []Event) []Event {
	for i := range s.products {
		p := &s.products[i]
		if p.Stock <= p.ReorderPoint {
			events = append(events, ProductRestocked{SKU: p.SKU, From: p.Stock, To: p.Capacity, At: s.now})
			p.Stock = p.Capacity
		}
	}
	return events
}

// prune drops the oldest terminal orders while more than MaxRetainedOrders
// are retained. Remaining orders keep placement order.
func (s *State) prune() {
	excess := len(s.orders) - MaxRetainedOrders
	if excess <= 0 {
		return
	}
	s.orders = slices.DeleteFunc(s.orders, func(rec *orderRecord) bool {
		if excess > 0 && rec.order.Status.Terminal() {
			excess--
			return true
		}
		return false
	})
}

// SetDependencyMode changes how a dependency behaves. It affects the next
// dependency call and the latency of the next stage entry; waits already
// running are not recalculated. Setting the current mode again is a no-op.
func (s *State) SetDependencyMode(name DependencyName, mode DependencyMode) error {
	if !slices.Contains(DependencyModes(), mode) {
		return fmt.Errorf("%w: %q", ErrInvalidDependencyMode, mode)
	}
	for i := range s.dependencies {
		if s.dependencies[i].Name == name {
			s.dependencies[i].Mode = mode
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrUnknownDependency, name)
}

// Snapshot returns a deep copy of the current state.
func (s *State) Snapshot() Snapshot {
	snap := Snapshot{
		Now:          s.now,
		Products:     slices.Clone(s.products),
		Orders:       make([]Order, 0, len(s.orders)),
		Dependencies: slices.Clone(s.dependencies),
	}
	for _, rec := range s.orders {
		snap.Orders = append(snap.Orders, rec.order.clone())
	}
	return snap
}

func (s *State) product(sku SKU) *Product {
	if i, ok := slices.BinarySearchFunc(s.products, sku, func(p Product, sku SKU) int {
		return strings.Compare(string(p.SKU), string(sku))
	}); ok {
		return &s.products[i]
	}
	return nil
}

func (s *State) mode(name DependencyName) DependencyMode {
	for _, d := range s.dependencies {
		if d.Name == name {
			return d.Mode
		}
	}
	panic(fmt.Sprintf("fulfillment: unknown dependency %q", name))
}

func latency(mode DependencyMode) int {
	if mode == ModeSlow {
		return SlowLatencyTicks
	}
	return HealthyLatencyTicks
}
