// Package inspected is a simulated order fulfillment service. It is the
// system that the Inspector will inspect.
//
// The service sells 5 products. Orders move through a payment stage and a
// shipping stage, each calling one downstream dependency (payment-gateway,
// warehouse) that can be healthy, slow or in outage. Time is a logical tick.
// The rules live in package fulfillment, the state is owned by the actor in
// package simulation, and metrics and health are derived from that state by
// packages metrics and health.
//
// # Path isolation
//
// Every endpoint lives under PathPrefix. The host mounts App.Handler at
// PathPrefix + "/" and does not strip the prefix.
//
// # Endpoints
//
//	GET  /inspected/                            index of links
//	GET  /inspected/health/live                 liveness: the simulation answers
//	GET  /inspected/health/ready                readiness with per-check reasons
//	GET  /inspected/metrics                     Prometheus text exposition
//	GET  /inspected/sim                         simulation status
//	PUT  /inspected/sim/clock                   pause or resume the clock
//	POST /inspected/sim/advance                 advance N ticks
//	PUT  /inspected/sim/dependencies/{name}     set a dependency mode
//	GET  /inspected/api/products                list products
//	GET  /inspected/api/products/{sku}          one product
//	GET  /inspected/api/orders                  list orders, filters status and sku
//	GET  /inspected/api/orders/{id}             one order with its history
//	POST /inspected/api/orders                  place an order
//
// The /inspected/sim endpoints control the simulation for the harness
// operator. They are separate from the observation endpoints, which are all
// read-only. Placing an order is the only business write.
//
// Products and orders carry links to related resources, and the index links
// to every collection, so a client can navigate from any resource.
//
// # JSON conventions
//
// Responses are JSON with snake_case names. An error has the form
//
//	{"error":{"code":"<code>","message":"<text>"}}
//
// with these codes:
//
//	invalid_request          400  malformed body, unknown field, missing field
//	invalid_status           400  status filter is not pending, paid, shipped or failed
//	unknown_dependency       404  no such dependency
//	product_not_found        404  no such product
//	order_not_found          404  no such order
//	invalid_advance          422  ticks outside 1..1000
//	invalid_dependency_mode  422  mode is not healthy, slow or outage
//	unknown_product          422  order for a SKU that does not exist
//	invalid_quantity         422  order quantity outside 1..10
//	simulation_unavailable   503  the simulation did not answer in time or stopped
//	internal_error           500  unexpected failure
//
// Requests to the simulation are bounded by Config.RequestTimeout. Delivery
// is at most once: a request that timed out may still have been applied.
//
// # Configuration
//
// Config has no defaults; every field is set by the host.
//
// # Lifecycle
//
// New wires the service and starts nothing. Run drives the wall-clock ticker
// and blocks until its context is cancelled. Run returns nil on cancellation
// and an error when the simulation fails. The host is the supervisor. State is
// in memory, so a restart is a new App that starts at tick 0.
package inspected
