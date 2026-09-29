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
// path prefix. It reads only /health/ready, /api/dependencies, /api/products
// and /api/orders, never the /sim control endpoints, and it does not import package
// inspected. It keeps its own copy of the JSON fields it needs.
//
// # Mapping
//
// Entities, in this order:
//
//	Source        Kind          ID         State             Reason                  Attributes
//	readiness     service       inspected  readiness status  none                    none
//	each check    health_check  name       status            check reason            none
//	each dep.     dependency    name       mode              none                    none
//	each product  product       sku        none              none                    name, price_cents, stock,
//	                                                                                 capacity, reorder_point
//	each order    order         id         status            last history reason     quantity, total_cents, channel,
//	                                                                                 placed_at_tick, updated_at_tick
//
// An order's History is its source history, oldest first, each entry with
// At written as "tick <n>". For a failed order the last history reason equals
// the source field failure_reason, so that field is not mapped separately. A
// history entry without a target status makes the order a gap.
//
// Relations, per item in entity order (Cause marks cause relations):
//
//	service/inspected    -has_check->    health_check/<name>  per check; cause when the check status equals the service status
//	health_check/<name>  -caused_by->    <target>             per link in the check causes; cause
//	order/<id>           -for_product->  product/<sku>        per order
//	order/<id>           -caused_by->    <target>             the cause link of the last history entry; cause
//	order/<id>           -waits_on->     <target>             the waiting_on link of an open order; cause
//
// The source documents the service status as the worst check status, so the
// checks with that status are exactly the ones that decide it. A link that
// ends in /api/dependencies/<name> targets dependency/<name>; one that ends in
// /api/products/<sku> targets product/<sku>; any other link is a gap.
//
// Readiness is accepted with status 200 and 503 (a down report is still an
// observation); dependencies, products and orders require 200.
//
// # Partial observation
//
// A failed read never hides the other reads. It becomes a gap of the
// snapshot, with the path as source and the error as text; a rejected status
// names the error code of the source when its body has one, for example
// "unexpected status 503: simulation_unavailable". Readiness without a status
// counts as a failed read. An item without a required field (check name or
// status, dependency name or mode, product sku, order id, sku or status) becomes a gap and is skipped;
// the other items of the document are kept. A relation whose target is
// missing becomes a gap when the read of the target succeeded, and is omitted
// silently when that read failed, since its gap already explains it. Only
// when every read fails does Observe return an error ("nothing observed").
//
// # Consistency
//
// The four reads are not atomic. Readiness, dependencies, products and orders can come
// from different simulation ticks. The snapshot is stamped with the clock
// passed to New after the last read.
package adapter
