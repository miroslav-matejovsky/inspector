package fulfillment_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

func newState(t *testing.T) *fulfillment.State {
	t.Helper()
	s, err := fulfillment.NewState(fulfillment.StandardCatalog())
	require.NoError(t, err)
	return s
}

func place(t *testing.T, s *fulfillment.State, sku fulfillment.SKU, qty int) fulfillment.Order {
	t.Helper()
	o, _, err := s.PlaceOrder(fulfillment.OrderRequest{SKU: sku, Quantity: qty}, fulfillment.ChannelAPI)
	require.NoError(t, err)
	return o
}

// tick runs n ticks and returns the events of every tick, one slice per tick.
func tick(s *fulfillment.State, n int) [][]fulfillment.Event {
	out := make([][]fulfillment.Event, 0, n)
	for range n {
		out = append(out, s.Tick())
	}
	return out
}

func orderOf(t *testing.T, s *fulfillment.State, id fulfillment.OrderID) fulfillment.Order {
	t.Helper()
	o, ok := s.Snapshot().Order(id)
	require.True(t, ok, "order %s not found", id)
	return o
}

func stockOf(t *testing.T, s *fulfillment.State, sku fulfillment.SKU) int {
	t.Helper()
	p, ok := s.Snapshot().Product(sku)
	require.True(t, ok, "product %s not found", sku)
	return p.Stock
}

func setMode(t *testing.T, s *fulfillment.State, name fulfillment.DependencyName, mode fulfillment.DependencyMode) {
	t.Helper()
	require.NoError(t, s.SetDependencyMode(name, mode))
}

func TestNewStateRejectsInvalidCatalog(t *testing.T) {
	valid := func() fulfillment.Product {
		return fulfillment.Product{SKU: "a", Name: "A", PriceCents: 100, Stock: 5, Capacity: 10, ReorderPoint: 2}
	}
	mutate := func(f func(p *fulfillment.Product)) []fulfillment.Product {
		p := valid()
		f(&p)
		return []fulfillment.Product{p}
	}

	tests := []struct {
		name    string
		catalog []fulfillment.Product
	}{
		{"empty", nil},
		{"duplicate sku", []fulfillment.Product{valid(), valid()}},
		{"empty sku", mutate(func(p *fulfillment.Product) { p.SKU = "" })},
		{"empty name", mutate(func(p *fulfillment.Product) { p.Name = "" })},
		{"zero price", mutate(func(p *fulfillment.Product) { p.PriceCents = 0 })},
		{"zero capacity", mutate(func(p *fulfillment.Product) { p.Capacity = 0 })},
		{"negative stock", mutate(func(p *fulfillment.Product) { p.Stock = -1 })},
		{"stock above capacity", mutate(func(p *fulfillment.Product) { p.Stock = 11 })},
		{"negative reorder point", mutate(func(p *fulfillment.Product) { p.ReorderPoint = -1 })},
		{"reorder point at capacity", mutate(func(p *fulfillment.Product) { p.ReorderPoint = 10 })},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fulfillment.NewState(tc.catalog)
			require.ErrorIs(t, err, fulfillment.ErrInvalidCatalog)
		})
	}
}

func TestNewStateCopiesCatalog(t *testing.T) {
	catalog := fulfillment.StandardCatalog()
	s, err := fulfillment.NewState(catalog)
	require.NoError(t, err)

	catalog[0].Stock = 1

	require.Equal(t, 40, s.Snapshot().Products[0].Stock)
}

func TestPlaceOrderValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     fulfillment.OrderRequest
		channel fulfillment.Channel
		want    error
	}{
		{"unknown sku", fulfillment.OrderRequest{SKU: "sku-999", Quantity: 1}, fulfillment.ChannelAPI, fulfillment.ErrUnknownProduct},
		{"zero quantity", fulfillment.OrderRequest{SKU: "sku-001", Quantity: 0}, fulfillment.ChannelAPI, fulfillment.ErrInvalidQuantity},
		{"quantity above max", fulfillment.OrderRequest{SKU: "sku-001", Quantity: 11}, fulfillment.ChannelAPI, fulfillment.ErrInvalidQuantity},
		{"unknown channel", fulfillment.OrderRequest{SKU: "sku-001", Quantity: 1}, "x", fulfillment.ErrInvalidChannel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newState(t)
			_, events, err := s.PlaceOrder(tc.req, tc.channel)
			require.ErrorIs(t, err, tc.want)
			require.Empty(t, events)
			require.Empty(t, s.Snapshot().Orders)
		})
	}
}

func TestPlaceOrderCreatesPendingOrder(t *testing.T) {
	s := newState(t)

	o, events, err := s.PlaceOrder(fulfillment.OrderRequest{SKU: "sku-001", Quantity: 2}, fulfillment.ChannelAPI)
	require.NoError(t, err)

	require.Equal(t, fulfillment.OrderID("ord-000001"), o.ID)
	require.Equal(t, int64(17800), o.TotalCents)
	require.Equal(t, fulfillment.StatusPending, o.Status)
	require.Equal(t, fulfillment.Tick(0), o.PlacedAt)
	require.Equal(t, []fulfillment.Transition{
		{From: "", To: fulfillment.StatusPending, At: 0, Reason: fulfillment.ReasonOrderPlaced},
	}, o.History)
	require.Equal(t, []fulfillment.Event{
		fulfillment.OrderPlaced{Order: "ord-000001", SKU: "sku-001", Quantity: 2, Channel: fulfillment.ChannelAPI, At: 0},
	}, events)
}

func TestHappyPath(t *testing.T) {
	s := newState(t)
	o := place(t, s, "sku-001", 2)

	events := tick(s, 2)

	require.Equal(t, []fulfillment.Event{
		fulfillment.DependencyCalled{Dependency: fulfillment.DependencyPaymentGateway, Succeeded: true, At: 1},
		fulfillment.OrderStatusChanged{
			Order: o.ID, SKU: "sku-001", From: fulfillment.StatusPending, To: fulfillment.StatusPaid,
			Reason: fulfillment.ReasonPaymentAuthorized, PlacedAt: 0, At: 1,
		},
	}, events[0])
	require.Equal(t, []fulfillment.Event{
		fulfillment.DependencyCalled{Dependency: fulfillment.DependencyWarehouse, Succeeded: true, At: 2},
		fulfillment.OrderStatusChanged{
			Order: o.ID, SKU: "sku-001", From: fulfillment.StatusPaid, To: fulfillment.StatusShipped,
			Reason: fulfillment.ReasonShipped, PlacedAt: 0, At: 2,
		},
	}, events[1])

	got := orderOf(t, s, o.ID)
	require.Equal(t, fulfillment.StatusShipped, got.Status)
	require.Len(t, got.History, 3)
	require.Equal(t, 38, stockOf(t, s, "sku-001"))
}

func TestPaymentDeclined(t *testing.T) {
	s := newState(t)
	o := place(t, s, "sku-003", 2) // 65800 > PaymentLimitCents

	tick(s, 1)

	got := orderOf(t, s, o.ID)
	require.Equal(t, fulfillment.StatusFailed, got.Status)
	require.Equal(t, fulfillment.ReasonPaymentDeclined, got.FailureReason)
	require.Equal(t, 10, stockOf(t, s, "sku-003"))
}

func TestPaymentGatewayOutage(t *testing.T) {
	s := newState(t)
	setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage)
	o := place(t, s, "sku-001", 1)

	events := tick(s, 3)

	failedCalls := 0
	for _, tickEvents := range events {
		for _, e := range tickEvents {
			if c, ok := e.(fulfillment.DependencyCalled); ok && c.Dependency == fulfillment.DependencyPaymentGateway && !c.Succeeded {
				failedCalls++
			}
		}
	}
	require.Equal(t, 3, failedCalls)

	got := orderOf(t, s, o.ID)
	require.Equal(t, fulfillment.StatusFailed, got.Status)
	require.Equal(t, fulfillment.ReasonPaymentGatewayUnavailable, got.FailureReason)
	require.Equal(t, fulfillment.Tick(3), got.UpdatedAt)
}

func TestPaymentGatewayOutageKeepsOrderPendingBeforeLastAttempt(t *testing.T) {
	s := newState(t)
	setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage)
	o := place(t, s, "sku-001", 1)

	tick(s, 1)
	require.Equal(t, fulfillment.StatusPending, orderOf(t, s, o.ID).Status)
	tick(s, 1)
	require.Equal(t, fulfillment.StatusPending, orderOf(t, s, o.ID).Status)
}

func TestPaymentGatewayRecoversBeforeLastAttempt(t *testing.T) {
	s := newState(t)
	setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage)
	o := place(t, s, "sku-001", 1)

	tick(s, 1)
	setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeHealthy)
	tick(s, 1)

	got := orderOf(t, s, o.ID)
	require.Equal(t, fulfillment.StatusPaid, got.Status)
	require.Equal(t, fulfillment.Tick(2), got.UpdatedAt)
}

func TestSlowPaymentGateway(t *testing.T) {
	s := newState(t)
	setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeSlow)
	o := place(t, s, "sku-001", 1)

	want := []fulfillment.OrderStatus{
		fulfillment.StatusPending, fulfillment.StatusPending, fulfillment.StatusPaid, fulfillment.StatusShipped,
	}
	for i, status := range want {
		tick(s, 1)
		require.Equal(t, status, orderOf(t, s, o.ID).Status, "after tick %d", i+1)
	}
}

func TestWarehouseOutage(t *testing.T) {
	s := newState(t)
	setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeOutage)
	o := place(t, s, "sku-001", 1)

	want := []fulfillment.OrderStatus{
		fulfillment.StatusPaid, fulfillment.StatusPaid, fulfillment.StatusPaid, fulfillment.StatusFailed,
	}
	for i, status := range want {
		tick(s, 1)
		require.Equal(t, status, orderOf(t, s, o.ID).Status, "after tick %d", i+1)
	}

	got := orderOf(t, s, o.ID)
	require.Equal(t, fulfillment.ReasonWarehouseUnavailable, got.FailureReason)
	require.Equal(t, []fulfillment.Tick{0, 1, 4}, []fulfillment.Tick{got.History[0].At, got.History[1].At, got.History[2].At})
}

// depleteSku003 places 11 single-unit orders for a product with stock 10 and
// runs the two ticks they need to reach a terminal status.
func depleteSku003(t *testing.T, s *fulfillment.State) {
	t.Helper()
	for range 11 {
		place(t, s, "sku-003", 1)
	}
	tick(s, 2)
}

func TestOutOfStock(t *testing.T) {
	s := newState(t)
	depleteSku003(t, s)

	for i := 1; i <= 10; i++ {
		id := fulfillment.OrderID(fmt.Sprintf("ord-%06d", i))
		require.Equal(t, fulfillment.StatusShipped, orderOf(t, s, id).Status, id)
	}
	last := orderOf(t, s, "ord-000011")
	require.Equal(t, fulfillment.StatusFailed, last.Status)
	require.Equal(t, fulfillment.ReasonOutOfStock, last.FailureReason)
	require.Equal(t, 0, stockOf(t, s, "sku-003"))
}

func restocks(events [][]fulfillment.Event) []fulfillment.ProductRestocked {
	var out []fulfillment.ProductRestocked
	for _, tickEvents := range events {
		for _, e := range tickEvents {
			if r, ok := e.(fulfillment.ProductRestocked); ok {
				out = append(out, r)
			}
		}
	}
	return out
}

func TestRestock(t *testing.T) {
	s := newState(t)
	depleteSku003(t, s)

	require.Empty(t, restocks(tick(s, 7))) // ticks 3..9

	tenth := tick(s, 1)
	require.Equal(t, []fulfillment.ProductRestocked{{SKU: "sku-003", From: 0, To: 10, At: 10}}, restocks(tenth))
	require.Equal(t, 10, stockOf(t, s, "sku-003"))
}

func TestRestockSkippedDuringWarehouseOutage(t *testing.T) {
	s := newState(t)
	depleteSku003(t, s)

	setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeOutage)
	require.Empty(t, restocks(tick(s, 8))) // ticks 3..10
	require.Equal(t, 0, stockOf(t, s, "sku-003"))

	setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeHealthy)
	got := restocks(tick(s, 10)) // ticks 11..20
	require.Equal(t, []fulfillment.ProductRestocked{{SKU: "sku-003", From: 0, To: 10, At: 20}}, got)
}

func TestSetDependencyMode(t *testing.T) {
	s := newState(t)

	require.ErrorIs(t, s.SetDependencyMode("unknown", fulfillment.ModeSlow), fulfillment.ErrUnknownDependency)
	require.ErrorIs(t, s.SetDependencyMode(fulfillment.DependencyWarehouse, "down"), fulfillment.ErrInvalidDependencyMode)

	require.NoError(t, s.SetDependencyMode(fulfillment.DependencyWarehouse, fulfillment.ModeSlow))
	require.Equal(t, []fulfillment.Dependency{
		{Name: fulfillment.DependencyPaymentGateway, Mode: fulfillment.ModeHealthy},
		{Name: fulfillment.DependencyWarehouse, Mode: fulfillment.ModeSlow},
	}, s.Snapshot().Dependencies)

	require.NoError(t, s.SetDependencyMode(fulfillment.DependencyWarehouse, fulfillment.ModeSlow))
}

func TestPruneRemovesOldestTerminalOrders(t *testing.T) {
	s := newState(t)
	for range fulfillment.MaxRetainedOrders {
		place(t, s, "sku-005", 1)
	}
	tick(s, 2) // every order is terminal: shipped or out_of_stock
	for range 5 {
		place(t, s, "sku-005", 1)
	}

	tick(s, 1)

	snap := s.Snapshot()
	require.Len(t, snap.Orders, fulfillment.MaxRetainedOrders)
	require.Equal(t, fulfillment.OrderID("ord-000006"), snap.Orders[0].ID)
	for _, id := range []fulfillment.OrderID{"ord-001001", "ord-001005"} {
		_, ok := snap.Order(id)
		require.True(t, ok, id)
	}
}

func TestPruneKeepsActiveOrders(t *testing.T) {
	s := newState(t)
	for range fulfillment.MaxRetainedOrders + 1 {
		place(t, s, "sku-005", 1)
	}

	tick(s, 1)

	snap := s.Snapshot()
	require.Len(t, snap.Orders, fulfillment.MaxRetainedOrders+1)
	for _, o := range snap.Orders {
		require.Equal(t, fulfillment.StatusPaid, o.Status)
	}
}

func TestSnapshotIsDeepCopy(t *testing.T) {
	s := newState(t)
	place(t, s, "sku-001", 1)

	snap := s.Snapshot()
	snap.Products[0].Stock = 0
	snap.Orders[0].History[0].Reason = "x"
	snap.Dependencies[0].Mode = fulfillment.ModeOutage

	again := s.Snapshot()
	require.Equal(t, 40, again.Products[0].Stock)
	require.Equal(t, fulfillment.ReasonOrderPlaced, again.Orders[0].History[0].Reason)
	require.Equal(t, fulfillment.ModeHealthy, again.Dependencies[0].Mode)
}

func TestPlaceOrderReturnsCopy(t *testing.T) {
	s := newState(t)
	o := place(t, s, "sku-001", 1)

	o.History[0].Reason = "x"

	require.Equal(t, fulfillment.ReasonOrderPlaced, orderOf(t, s, o.ID).History[0].Reason)
}

func TestSnapshotLookup(t *testing.T) {
	s := newState(t)
	place(t, s, "sku-001", 1)
	snap := s.Snapshot()

	_, ok := snap.Product("sku-001")
	require.True(t, ok)
	_, ok = snap.Product("sku-999")
	require.False(t, ok)
	_, ok = snap.Order("ord-000001")
	require.True(t, ok)
	_, ok = snap.Order("ord-999999")
	require.False(t, ok)
}

func TestStateIsDeterministic(t *testing.T) {
	run := func() (fulfillment.Snapshot, [][]fulfillment.Event) {
		s := newState(t)
		var events [][]fulfillment.Event
		skus := []fulfillment.SKU{"sku-001", "sku-002", "sku-003"}
		for i := range 20 {
			_, placed, err := s.PlaceOrder(
				fulfillment.OrderRequest{SKU: skus[i%3], Quantity: 1 + i%4}, fulfillment.ChannelSimulation)
			require.NoError(t, err)
			events = append(events, placed)
			switch i {
			case 5:
				setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeSlow)
			case 10:
				setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeOutage)
			case 15:
				setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeHealthy)
			}
			events = append(events, tick(s, 1)...)
		}
		events = append(events, tick(s, 5)...)
		return s.Snapshot(), events
	}

	snapA, eventsA := run()
	snapB, eventsB := run()

	require.Equal(t, snapA, snapB)
	require.Equal(t, eventsA, eventsB)
}

func TestTransitionCauses(t *testing.T) {
	gateway := fulfillment.Cause{Dependency: fulfillment.DependencyPaymentGateway}
	warehouse := fulfillment.Cause{Dependency: fulfillment.DependencyWarehouse}
	tests := map[string]struct {
		run    func(t *testing.T, s *fulfillment.State) fulfillment.OrderID
		causes []fulfillment.Cause // expected cause per history entry
	}{
		"authorized and shipped": {
			run: func(t *testing.T, s *fulfillment.State) fulfillment.OrderID {
				o := place(t, s, "sku-001", 1)
				tick(s, 2)
				return o.ID
			},
			causes: []fulfillment.Cause{{}, gateway, warehouse},
		},
		"payment declined": {
			run: func(t *testing.T, s *fulfillment.State) fulfillment.OrderID {
				o := place(t, s, "sku-003", 2) // 65800 cents, above the limit
				tick(s, 1)
				return o.ID
			},
			causes: []fulfillment.Cause{{}, {}},
		},
		"payment gateway unavailable": {
			run: func(t *testing.T, s *fulfillment.State) fulfillment.OrderID {
				setMode(t, s, fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage)
				o := place(t, s, "sku-001", 1)
				tick(s, 3)
				return o.ID
			},
			causes: []fulfillment.Cause{{}, gateway},
		},
		"warehouse unavailable": {
			run: func(t *testing.T, s *fulfillment.State) fulfillment.OrderID {
				o := place(t, s, "sku-001", 1)
				tick(s, 1)
				setMode(t, s, fulfillment.DependencyWarehouse, fulfillment.ModeOutage)
				tick(s, 3)
				return o.ID
			},
			causes: []fulfillment.Cause{{}, gateway, warehouse},
		},
		"out of stock": {
			run: func(t *testing.T, s *fulfillment.State) fulfillment.OrderID {
				depleteSku003(t, s)
				return "ord-000011"
			},
			causes: []fulfillment.Cause{{}, gateway, {Product: "sku-003"}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newState(t)

			o := orderOf(t, s, tc.run(t, s))

			var got []fulfillment.Cause
			for _, tr := range o.History {
				got = append(got, tr.Cause)
			}
			require.Equal(t, tc.causes, got)
		})
	}
}

func TestStageDependency(t *testing.T) {
	tests := map[fulfillment.OrderStatus]struct {
		dep fulfillment.DependencyName
		ok  bool
	}{
		fulfillment.StatusPending: {fulfillment.DependencyPaymentGateway, true},
		fulfillment.StatusPaid:    {fulfillment.DependencyWarehouse, true},
		fulfillment.StatusShipped: {"", false},
		fulfillment.StatusFailed:  {"", false},
	}
	for status, want := range tests {
		dep, ok := status.StageDependency()

		require.Equal(t, want.dep, dep, status)
		require.Equal(t, want.ok, ok, status)
	}
}
