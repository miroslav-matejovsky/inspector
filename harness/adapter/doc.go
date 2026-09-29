// Package adapter maps the inspected simulation into the Inspector
// observation model. It is the only package that knows both the JSON
// contract of the inspected service and package observation, so all
// simulated domain vocabulary Inspector shows is defined here, in the
// harness, and never in the Inspector library.
//
// # Boundary
//
// The adapter reads the inspected service like any external client: GET
// requests through connectivity.HTTPReader, whose base URL is the inspected
// path prefix. It reads only /health/ready, /api/products and /api/orders,
// never the /sim control endpoints, and it does not import package
// inspected. It keeps its own copy of the JSON fields it needs.
//
// # Mapping
//
// Entities, in this order:
//
//	Source        Kind          ID         State             Attributes
//	readiness     service       inspected  readiness status  none
//	each check    health_check  name       status            reason, when not empty
//	each product  product       sku        none              name, price_cents, stock, capacity, reorder_point
//	each order    order         id         status            quantity, total_cents, channel, placed_at_tick,
//	                                                         updated_at_tick, failure_reason when not empty
//
// Relations, in this order:
//
//	service/inspected  -has_check->    health_check/<name>  one per check
//	order/<id>         -for_product->  product/<sku>        one per order
//
// Readiness is accepted with status 200 and 503 (a down report is still an
// observation); products and orders require 200. A missing required field
// (readiness status, check name and status, product sku, order id, sku and
// status) is an error.
//
// # Consistency
//
// The three reads are not atomic. Readiness, products and orders can come
// from different simulation ticks. The snapshot is stamped with the clock
// passed to New after the last read.
package adapter
