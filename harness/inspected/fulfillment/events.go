package fulfillment

// Event is a fact produced by a State mutation. The set of event types is
// closed: only this package implements Event.
type Event interface{ isEvent() }

// OrderPlaced reports an accepted order.
type OrderPlaced struct {
	Order    OrderID
	SKU      SKU
	Quantity int
	Channel  Channel
	At       Tick
}

// OrderStatusChanged reports an order status transition. PlacedAt allows a
// consumer to compute how long the order took.
type OrderStatusChanged struct {
	Order    OrderID
	SKU      SKU
	From     OrderStatus
	To       OrderStatus
	Reason   Reason
	PlacedAt Tick
	At       Tick
}

// DependencyCalled reports one call to a downstream dependency.
type DependencyCalled struct {
	Dependency DependencyName
	Succeeded  bool
	At         Tick
}

// ProductRestocked reports a warehouse restock.
type ProductRestocked struct {
	SKU  SKU
	From int
	To   int
	At   Tick
}

func (OrderPlaced) isEvent()        {}
func (OrderStatusChanged) isEvent() {}
func (DependencyCalled) isEvent()   {}
func (ProductRestocked) isEvent()   {}
