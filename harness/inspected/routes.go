package inspected

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Route paths, relative to PathPrefix.
const (
	pathHealthLive    = "/health/live"
	pathHealthReady   = "/health/ready"
	pathMetrics       = "/metrics"
	pathSim           = "/sim"
	pathSimClock      = "/sim/clock"
	pathSimAdvance    = "/sim/advance"
	pathSimDependency = "/sim/dependencies/{name}"
	pathProducts      = "/api/products"
	pathProduct       = pathProducts + "/{sku}"
	pathOrders        = "/api/orders"
	pathOrder         = pathOrders + "/{id}"
)

// route is one endpoint. name is the handler label of its HTTP metrics.
type route struct {
	pattern string
	name    string
	handler http.Handler
}

// pattern builds a ServeMux pattern under PathPrefix.
func pattern(method, path string) string {
	return method + " " + PathPrefix + path
}

func (a *App) routes() []route {
	return []route{
		{"GET " + PathPrefix + "/{$}", "index", http.HandlerFunc(a.handleIndex)},
		{pattern(http.MethodGet, pathHealthLive), "health_live", http.HandlerFunc(a.handleLive)},
		{pattern(http.MethodGet, pathHealthReady), "health_ready", http.HandlerFunc(a.handleReady)},
		{pattern(http.MethodGet, pathMetrics), "metrics", promhttp.HandlerFor(a.registry, promhttp.HandlerOpts{
			ErrorHandling: promhttp.HTTPErrorOnError,
		})},
		{pattern(http.MethodGet, pathSim), "sim_status", http.HandlerFunc(a.handleSimStatus)},
		{pattern(http.MethodPut, pathSimClock), "sim_clock", http.HandlerFunc(a.handleSimClock)},
		{pattern(http.MethodPost, pathSimAdvance), "sim_advance", http.HandlerFunc(a.handleSimAdvance)},
		{pattern(http.MethodPut, pathSimDependency), "sim_dependency", http.HandlerFunc(a.handleSimDependency)},
		{pattern(http.MethodGet, pathProducts), "products", http.HandlerFunc(a.handleListProducts)},
		{pattern(http.MethodGet, pathProduct), "product", http.HandlerFunc(a.handleGetProduct)},
		{pattern(http.MethodGet, pathOrders), "orders", http.HandlerFunc(a.handleListOrders)},
		{pattern(http.MethodGet, pathOrder), "order", http.HandlerFunc(a.handleGetOrder)},
		{pattern(http.MethodPost, pathOrders), "place_order", http.HandlerFunc(a.handlePlaceOrder)},
	}
}
