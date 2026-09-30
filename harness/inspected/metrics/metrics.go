package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

const (
	resultSuccess = "success"
	resultFailure = "failure"
)

// Metrics owns the metric collectors of the inspected service.
//
// Record and Observe are called by the simulation actor only.
// InstrumentHandler wrappers run on HTTP goroutines; the prometheus
// collectors are internally synchronized.
type Metrics struct {
	ordersPlaced     *prometheus.CounterVec
	transitions      *prometheus.CounterVec
	orderDuration    *prometheus.HistogramVec
	dependencyCalls  *prometheus.CounterVec
	restocks         *prometheus.CounterVec
	ordersInProgress *prometheus.GaugeVec
	productStock     *prometheus.GaugeVec
	dependencyUp     *prometheus.GaugeVec
	dependencyMode   *prometheus.GaugeVec
	tick             prometheus.Gauge
	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
}

// inProgressStatuses are the non-terminal order statuses.
var inProgressStatuses = []fulfillment.OrderStatus{fulfillment.StatusPending, fulfillment.StatusPaid}

// terminalStatuses are the terminal order statuses.
var terminalStatuses = []fulfillment.OrderStatus{fulfillment.StatusShipped, fulfillment.StatusFailed}

// New creates all collectors and registers them on reg. Known series are
// created with value 0, see the package documentation.
func New(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		ordersPlaced: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fulfillment_orders_placed_total",
			Help: "Orders accepted by the fulfillment service, by placement channel.",
		}, []string{"channel"}),
		transitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fulfillment_order_transitions_total",
			Help: "Order status transitions, by source status, target status and reason.",
		}, []string{"from", "to", "reason"}),
		orderDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "fulfillment_order_duration_ticks",
			Help:    "Simulation ticks from order placement to a terminal status.",
			Buckets: []float64{1, 2, 3, 4, 6, 8, 12},
		}, []string{"status"}),
		dependencyCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fulfillment_dependency_calls_total",
			Help: "Calls to downstream dependencies, by dependency and result.",
		}, []string{"dependency", "result"}),
		restocks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fulfillment_product_restocks_total",
			Help: "Warehouse restocks, by product SKU.",
		}, []string{"sku"}),
		ordersInProgress: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fulfillment_orders_in_progress",
			Help: "Orders not yet in a terminal status, by status.",
		}, []string{"status"}),
		productStock: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fulfillment_product_stock",
			Help: "Units in stock, by product SKU.",
		}, []string{"sku"}),
		dependencyUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fulfillment_dependency_up",
			Help: "Whether a downstream dependency accepts calls (1) or is in outage (0).",
		}, []string{"dependency"}),
		dependencyMode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "fulfillment_dependency_mode",
			Help: "Current simulated mode of a downstream dependency, 1 for the active mode and 0 otherwise.",
		}, []string{"dependency", "mode"}),
		tick: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fulfillment_simulation_tick",
			Help: "Current simulation tick.",
		}),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "inspected_http_requests_total",
			Help: "HTTP requests handled by the inspected service, by handler, status code and method.",
		}, []string{"handler", "code", "method"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "inspected_http_request_duration_seconds",
			Help:    "HTTP request latency of the inspected service, by handler and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"handler", "method"}),
	}

	collectors := []prometheus.Collector{
		m.ordersPlaced, m.transitions, m.orderDuration, m.dependencyCalls, m.restocks,
		m.ordersInProgress, m.productStock, m.dependencyUp, m.dependencyMode, m.tick,
		m.httpRequests, m.httpDuration,
	}
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("metrics: register collector: %w", err)
		}
	}

	m.initSeries()
	return m, nil
}

// initSeries creates every series whose label values are known up front, so
// that they are visible from the first scrape.
func (m *Metrics) initSeries() {
	for _, c := range fulfillment.Channels() {
		m.ordersPlaced.WithLabelValues(string(c))
	}
	for _, r := range fulfillment.AllowedTransitions() {
		m.transitions.WithLabelValues(string(r.From), string(r.To), string(r.Reason))
	}
	for _, s := range terminalStatuses {
		m.orderDuration.WithLabelValues(string(s))
	}
	for _, d := range fulfillment.DependencyNames() {
		m.dependencyCalls.WithLabelValues(string(d), resultSuccess)
		m.dependencyCalls.WithLabelValues(string(d), resultFailure)
	}
	for _, s := range inProgressStatuses {
		m.ordersInProgress.WithLabelValues(string(s))
	}
}

// Record counts events. It panics on an event type it does not know: the
// event set is closed in package fulfillment, so that is a programming error.
func (m *Metrics) Record(events []fulfillment.Event) {
	for _, e := range events {
		switch e := e.(type) {
		case fulfillment.OrderPlaced:
			m.ordersPlaced.WithLabelValues(string(e.Channel)).Inc()
		case fulfillment.OrderStatusChanged:
			m.transitions.WithLabelValues(string(e.From), string(e.To), string(e.Reason)).Inc()
			if e.To.Terminal() {
				m.orderDuration.WithLabelValues(string(e.To)).Observe(float64(e.At - e.PlacedAt))
			}
		case fulfillment.DependencyCalled:
			result := resultFailure
			if e.Succeeded {
				result = resultSuccess
			}
			m.dependencyCalls.WithLabelValues(string(e.Dependency), result).Inc()
		case fulfillment.ProductRestocked:
			m.restocks.WithLabelValues(string(e.SKU)).Inc()
		default:
			panic(fmt.Sprintf("metrics: unhandled event %T", e))
		}
	}
}

// Observe sets every gauge from a snapshot.
func (m *Metrics) Observe(s fulfillment.Snapshot) {
	m.tick.Set(float64(s.Now))

	inProgress := map[fulfillment.OrderStatus]int{}
	for _, o := range s.Orders {
		inProgress[o.Status]++
	}
	for _, status := range inProgressStatuses {
		m.ordersInProgress.WithLabelValues(string(status)).Set(float64(inProgress[status]))
	}

	for _, p := range s.Products {
		m.productStock.WithLabelValues(string(p.SKU)).Set(float64(p.Stock))
	}

	for _, d := range s.Dependencies {
		up := 1.0
		if d.Mode == fulfillment.ModeOutage {
			up = 0
		}
		m.dependencyUp.WithLabelValues(string(d.Name)).Set(up)
		for _, mode := range fulfillment.DependencyModes() {
			active := 0.0
			if d.Mode == mode {
				active = 1
			}
			m.dependencyMode.WithLabelValues(string(d.Name), string(mode)).Set(active)
		}
	}
}

// InstrumentHandler counts and times the requests served by h under the
// handler label name.
func (m *Metrics) InstrumentHandler(name string, h http.Handler) http.Handler {
	labels := prometheus.Labels{"handler": name}
	return promhttp.InstrumentHandlerDuration(m.httpDuration.MustCurryWith(labels),
		promhttp.InstrumentHandlerCounter(m.httpRequests.MustCurryWith(labels), h))
}
