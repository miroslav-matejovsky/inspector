// Package metrics defines the Prometheus metrics of the inspected service.
//
// Metrics registers its collectors on a registerer passed by the caller and
// never touches the global default registry. It turns fulfillment events into
// counters and histograms (Record), turns a fulfillment snapshot into gauges
// (Observe) and instruments HTTP handlers (InstrumentHandler). Record and
// Observe match the simulation.Recorder interface, which is declared by the
// consumer.
//
// # Metric catalog
//
//	fulfillment_orders_placed_total{channel}                counter
//	    Orders accepted, by placement channel.
//	fulfillment_order_transitions_total{from,to,reason}     counter
//	    Order status transitions with their reason.
//	fulfillment_order_duration_ticks{status}                histogram
//	    Ticks from placement to a terminal status (shipped, failed).
//	fulfillment_dependency_calls_total{dependency,result}   counter
//	    Calls to payment-gateway and warehouse, result success or failure.
//	fulfillment_product_restocks_total{sku}                 counter
//	    Warehouse restocks, by SKU.
//	fulfillment_orders_in_progress{status}                  gauge
//	    Orders in status pending or paid.
//	fulfillment_product_stock{sku}                          gauge
//	    Units in stock, by SKU.
//	fulfillment_dependency_up{dependency}                   gauge
//	    1 when the dependency accepts calls, 0 during an outage.
//	fulfillment_dependency_mode{dependency,mode}            gauge
//	    1 for the active mode (healthy, slow, outage), 0 for the others.
//	fulfillment_simulation_tick                             gauge
//	    Current simulation tick.
//	inspected_http_requests_total{handler,code,method}      counter
//	    HTTP requests, method in lower case.
//	inspected_http_request_duration_seconds{handler,method} histogram
//	    HTTP request latency.
//
// # Series initialization
//
// New creates the series whose labels are known up front with value 0: the
// placement channels, the allowed order transitions, the dependency call
// results, and the terminal and in-progress statuses. Series that depend on
// the catalog or on the dependencies, such as stock and dependency gauges,
// appear with the first Observe. Restock series appear with the first
// restock of a SKU.
//
// # Concurrency
//
// The prometheus collectors are internally synchronized. By convention Record
// and Observe are called from the simulation actor only.
package metrics
