// Package health evaluates a fulfillment snapshot into a readiness report.
//
// Evaluate is a pure function. Every check that is not up carries a reason
// that names its cause, so the report explains itself.
//
// # Checks
//
// One check per dependency, named by the dependency name:
//
//	healthy -> up
//	slow    -> degraded, "<name> is slow: calls take <n> ticks"
//	outage  -> down,     "<name> is in outage: calls fail"
//
// One check named "inventory":
//
//	every product in stock       -> up
//	some products out of stock   -> degraded, "out of stock: <sku>, <sku>"
//
// # Overall status
//
// The report status is the worst check status: down, then degraded, then up.
// The HTTP mapping is done by the caller: down is unavailable, up and
// degraded are available.
package health
