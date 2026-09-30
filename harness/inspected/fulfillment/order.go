package fulfillment

import "fmt"

// OrderID identifies an order, format "ord-%06d" with a sequence starting at 1.
type OrderID string

// OrderStatus is the lifecycle status of an order.
type OrderStatus string

// Order statuses. shipped and failed are terminal.
const (
	StatusPending OrderStatus = "pending"
	StatusPaid    OrderStatus = "paid"
	StatusShipped OrderStatus = "shipped"
	StatusFailed  OrderStatus = "failed"
)

// Terminal reports whether no further transition is possible.
func (s OrderStatus) Terminal() bool {
	return s == StatusShipped || s == StatusFailed
}

// ParseOrderStatus converts s to an OrderStatus. An unknown value returns an
// error wrapping ErrInvalidOrderStatus.
func ParseOrderStatus(s string) (OrderStatus, error) {
	switch status := OrderStatus(s); status {
	case StatusPending, StatusPaid, StatusShipped, StatusFailed:
		return status, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidOrderStatus, s)
}

// Channel is the way an order entered the system.
type Channel string

// Order channels.
const (
	ChannelAPI        Channel = "api"        // placed through the business API
	ChannelSimulation Channel = "simulation" // placed by the traffic generator
)

// Channels returns every channel. Every call returns a new slice.
func Channels() []Channel {
	return []Channel{ChannelAPI, ChannelSimulation}
}

// Reason states why an order changed status.
type Reason string

// Transition reasons.
const (
	ReasonOrderPlaced               Reason = "order_placed"
	ReasonPaymentAuthorized         Reason = "payment_authorized"
	ReasonPaymentDeclined           Reason = "payment_declined"
	ReasonPaymentGatewayUnavailable Reason = "payment_gateway_unavailable"
	ReasonShipped                   Reason = "shipped"
	ReasonOutOfStock                Reason = "out_of_stock"
	ReasonWarehouseUnavailable      Reason = "warehouse_unavailable"
)

// StageDependency returns the dependency an order in status s waits for:
// payment-gateway while pending, warehouse while paid. Terminal statuses wait
// for nothing and return false.
func (s OrderStatus) StageDependency() (DependencyName, bool) {
	switch s {
	case StatusPending:
		return DependencyPaymentGateway, true
	case StatusPaid:
		return DependencyWarehouse, true
	}
	return "", false
}

// Cause names what decided a transition, as known when it happened. At most
// one field is set. Both are empty when the order itself decided: its
// placement, or a payment declined because TotalCents exceeds PaymentLimitCents.
type Cause struct {
	Dependency DependencyName // the dependency whose call decided the transition
	Product    SKU            // the product whose stock decided the transition
}

// Transition is one entry of an order history.
type Transition struct {
	From   OrderStatus // empty for the initial entry
	To     OrderStatus
	At     Tick
	Reason Reason
	Cause  Cause
}

// TransitionRule is a status change the domain allows.
type TransitionRule struct {
	From   OrderStatus
	To     OrderStatus
	Reason Reason
}

// AllowedTransitions returns every allowed status change, excluding the
// initial placement. Every call returns a new slice.
func AllowedTransitions() []TransitionRule {
	return []TransitionRule{
		{From: StatusPending, To: StatusPaid, Reason: ReasonPaymentAuthorized},
		{From: StatusPending, To: StatusFailed, Reason: ReasonPaymentDeclined},
		{From: StatusPending, To: StatusFailed, Reason: ReasonPaymentGatewayUnavailable},
		{From: StatusPaid, To: StatusShipped, Reason: ReasonShipped},
		{From: StatusPaid, To: StatusFailed, Reason: ReasonOutOfStock},
		{From: StatusPaid, To: StatusFailed, Reason: ReasonWarehouseUnavailable},
	}
}

// OrderRequest asks for Quantity units of one product.
type OrderRequest struct {
	SKU      SKU
	Quantity int
}

// Order is a request for units of one product with its full history.
type Order struct {
	ID            OrderID
	SKU           SKU
	Quantity      int
	TotalCents    int64 // price at placement times quantity
	Channel       Channel
	Status        OrderStatus
	PlacedAt      Tick
	UpdatedAt     Tick
	FailureReason Reason // empty unless Status is failed
	History       []Transition
}

func (o Order) clone() Order {
	o.History = append([]Transition(nil), o.History...)
	return o
}
