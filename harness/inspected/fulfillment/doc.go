// Package fulfillment holds the business rules of the simulated order
// fulfillment service.
//
// The package is a pure, deterministic state machine. It has no goroutines, no
// I/O, no wall clock and no randomness. A State is not safe for concurrent
// use; exactly one owner mutates it.
//
// # Time
//
// Time is a logical Tick. A new State is at tick 0. State.Tick moves time one
// step forward and returns the resulting events. All durations are counted in
// ticks.
//
// # Order lifecycle
//
//	           payment_authorized            shipped
//	pending  ----------------------> paid ------------> shipped
//	   |                               |
//	   | payment_declined              | out_of_stock
//	   | payment_gateway_unavailable   | warehouse_unavailable
//	   v                               v
//	 failed                          failed
//
// An order is placed in status pending. While pending, it calls the
// payment-gateway dependency. While paid, it calls the warehouse dependency.
// Before its stage calls the dependency, the order waits HealthyLatencyTicks
// (SlowLatencyTicks when the dependency is slow). Every transition is stored
// in Order.History with its tick and Reason and is reported as an
// OrderStatusChanged event.
//
// # Causes
//
// Every history entry records in Transition.Cause what decided it, as known
// when it happened: the payment-gateway for payment_authorized and
// payment_gateway_unavailable, the warehouse for shipped and
// warehouse_unavailable, the order's product for out_of_stock. Placement and
// payment_declined (the order's own total) have no cause. An open order waits
// on the dependency of its stage, see OrderStatus.StageDependency.
//
// # Dependencies
//
// A dependency is healthy, slow or in outage. During an outage a call fails
// and the stage is retried after HealthyLatencyTicks. After MaxStageAttempts
// failed calls in one stage the order fails with
// ReasonPaymentGatewayUnavailable or ReasonWarehouseUnavailable.
//
// # Stock
//
// Stock is checked and consumed when the warehouse call succeeds, not when the
// order is placed. Every RestockIntervalTicks ticks, and only while the
// warehouse is not in outage, each product with Stock <= ReorderPoint is
// refilled to Capacity.
//
// # Payment
//
// An order whose TotalCents exceeds PaymentLimitCents is declined.
//
// # Retention
//
// At the end of every tick the oldest terminal orders above MaxRetainedOrders
// are dropped. Orders that are not terminal are never dropped.
//
// # Events
//
// Event is a closed set: OrderPlaced, OrderStatusChanged, DependencyCalled and
// ProductRestocked. Events are returned in the order they happened.
package fulfillment
