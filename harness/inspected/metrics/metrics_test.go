package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/metrics"
)

func newMetrics(t *testing.T) (*metrics.Metrics, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m, err := metrics.New(reg)
	require.NoError(t, err)
	return m, reg
}

// find returns the series of the named metric whose labels equal want exactly.
func find(t *testing.T, reg *prometheus.Registry, name string, want map[string]string) *dto.Metric {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
			if len(got) == len(want) && matches(got, want) {
				return m
			}
		}
	}
	require.FailNow(t, "series not found", "%s%v", name, want)
	return nil
}

func matches(got, want map[string]string) bool {
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func counter(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	return find(t, reg, name, labels).GetCounter().GetValue()
}

func gauge(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	return find(t, reg, name, labels).GetGauge().GetValue()
}

func l(kv ...string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func TestNewInitializesKnownSeries(t *testing.T) {
	_, reg := newMetrics(t)

	expected := `
# HELP fulfillment_orders_placed_total Orders accepted by the fulfillment service, by placement channel.
# TYPE fulfillment_orders_placed_total counter
fulfillment_orders_placed_total{channel="api"} 0
fulfillment_orders_placed_total{channel="simulation"} 0
`
	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expected), "fulfillment_orders_placed_total"))

	count, err := testutil.GatherAndCount(reg, "fulfillment_order_transitions_total")
	require.NoError(t, err)
	require.Equal(t, 6, count)

	count, err = testutil.GatherAndCount(reg, "fulfillment_dependency_calls_total")
	require.NoError(t, err)
	require.Equal(t, 4, count)

	count, err = testutil.GatherAndCount(reg, "fulfillment_orders_in_progress")
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = testutil.GatherAndCount(reg, "fulfillment_order_duration_ticks")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestNewFailsOnDuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	_, err := metrics.New(reg)
	require.NoError(t, err)

	_, err = metrics.New(reg)

	require.Error(t, err)
}

func TestRecordEvents(t *testing.T) {
	m, reg := newMetrics(t)

	m.Record([]fulfillment.Event{
		fulfillment.OrderPlaced{Order: "ord-000001", SKU: "sku-001", Quantity: 1, Channel: fulfillment.ChannelAPI, At: 0},
		fulfillment.DependencyCalled{Dependency: fulfillment.DependencyPaymentGateway, Succeeded: true, At: 1},
		fulfillment.OrderStatusChanged{
			Order: "ord-000001", SKU: "sku-001", From: fulfillment.StatusPending, To: fulfillment.StatusPaid,
			Reason: fulfillment.ReasonPaymentAuthorized, PlacedAt: 0, At: 1,
		},
		fulfillment.DependencyCalled{Dependency: fulfillment.DependencyWarehouse, Succeeded: false, At: 2},
		fulfillment.ProductRestocked{SKU: "sku-003", From: 0, To: 10, At: 10},
	})

	require.Equal(t, 1.0, counter(t, reg, "fulfillment_orders_placed_total", l("channel", "api")))
	require.Equal(t, 0.0, counter(t, reg, "fulfillment_orders_placed_total", l("channel", "simulation")))

	transitions := "fulfillment_order_transitions_total"
	require.Equal(t, 1.0, counter(t, reg, transitions, l("from", "pending", "to", "paid", "reason", "payment_authorized")))
	for _, r := range fulfillment.AllowedTransitions()[1:] {
		got := counter(t, reg, transitions, l("from", string(r.From), "to", string(r.To), "reason", string(r.Reason)))
		require.Equal(t, 0.0, got, r)
	}

	calls := "fulfillment_dependency_calls_total"
	require.Equal(t, 1.0, counter(t, reg, calls, l("dependency", "payment-gateway", "result", "success")))
	require.Equal(t, 1.0, counter(t, reg, calls, l("dependency", "warehouse", "result", "failure")))
	require.Equal(t, 0.0, counter(t, reg, calls, l("dependency", "payment-gateway", "result", "failure")))
	require.Equal(t, 0.0, counter(t, reg, calls, l("dependency", "warehouse", "result", "success")))

	require.Equal(t, 1.0, counter(t, reg, "fulfillment_product_restocks_total", l("sku", "sku-003")))
}

func TestRecordTerminalDuration(t *testing.T) {
	m, reg := newMetrics(t)

	m.Record([]fulfillment.Event{
		fulfillment.OrderStatusChanged{
			Order: "ord-000001", SKU: "sku-001", From: fulfillment.StatusPaid, To: fulfillment.StatusShipped,
			Reason: fulfillment.ReasonShipped, PlacedAt: 0, At: 2,
		},
	})

	h := find(t, reg, "fulfillment_order_duration_ticks", l("status", "shipped")).GetHistogram()
	require.Equal(t, uint64(1), h.GetSampleCount())
	require.Equal(t, 2.0, h.GetSampleSum())
	buckets := map[float64]uint64{}
	for _, b := range h.GetBucket() {
		buckets[b.GetUpperBound()] = b.GetCumulativeCount()
	}
	require.Equal(t, uint64(0), buckets[1])
	require.Equal(t, uint64(1), buckets[2])

	failed := find(t, reg, "fulfillment_order_duration_ticks", l("status", "failed")).GetHistogram()
	require.Equal(t, uint64(0), failed.GetSampleCount())
}

func TestRecordIgnoresNonTerminalDuration(t *testing.T) {
	m, reg := newMetrics(t)

	m.Record([]fulfillment.Event{
		fulfillment.OrderStatusChanged{
			Order: "ord-000001", SKU: "sku-001", From: fulfillment.StatusPending, To: fulfillment.StatusPaid,
			Reason: fulfillment.ReasonPaymentAuthorized, PlacedAt: 0, At: 1,
		},
	})

	for _, status := range []string{"shipped", "failed"} {
		h := find(t, reg, "fulfillment_order_duration_ticks", l("status", status)).GetHistogram()
		require.Equal(t, uint64(0), h.GetSampleCount(), status)
	}
}

func TestObserveSnapshot(t *testing.T) {
	m, reg := newMetrics(t)
	state, err := fulfillment.NewState(fulfillment.StandardCatalog())
	require.NoError(t, err)
	require.NoError(t, state.SetDependencyMode(fulfillment.DependencyPaymentGateway, fulfillment.ModeOutage))
	_, _, err = state.PlaceOrder(fulfillment.OrderRequest{SKU: "sku-001", Quantity: 1}, fulfillment.ChannelAPI)
	require.NoError(t, err)

	m.Observe(state.Snapshot())

	require.Equal(t, 0.0, gauge(t, reg, "fulfillment_simulation_tick", l()))
	require.Equal(t, 1.0, gauge(t, reg, "fulfillment_orders_in_progress", l("status", "pending")))
	require.Equal(t, 0.0, gauge(t, reg, "fulfillment_orders_in_progress", l("status", "paid")))
	require.Equal(t, 40.0, gauge(t, reg, "fulfillment_product_stock", l("sku", "sku-001")))
	count, err := testutil.GatherAndCount(reg, "fulfillment_product_stock")
	require.NoError(t, err)
	require.Equal(t, 5, count)

	require.Equal(t, 0.0, gauge(t, reg, "fulfillment_dependency_up", l("dependency", "payment-gateway")))
	require.Equal(t, 1.0, gauge(t, reg, "fulfillment_dependency_up", l("dependency", "warehouse")))

	mode := "fulfillment_dependency_mode"
	require.Equal(t, 1.0, gauge(t, reg, mode, l("dependency", "payment-gateway", "mode", "outage")))
	require.Equal(t, 0.0, gauge(t, reg, mode, l("dependency", "payment-gateway", "mode", "healthy")))
	require.Equal(t, 0.0, gauge(t, reg, mode, l("dependency", "payment-gateway", "mode", "slow")))
	count, err = testutil.GatherAndCount(reg, mode)
	require.NoError(t, err)
	require.Equal(t, 6, count)
}

func TestObserveFollowsStateChanges(t *testing.T) {
	m, reg := newMetrics(t)
	state, err := fulfillment.NewState(fulfillment.StandardCatalog())
	require.NoError(t, err)
	_, _, err = state.PlaceOrder(fulfillment.OrderRequest{SKU: "sku-001", Quantity: 2}, fulfillment.ChannelAPI)
	require.NoError(t, err)

	state.Tick()
	m.Observe(state.Snapshot())
	require.Equal(t, 1.0, gauge(t, reg, "fulfillment_simulation_tick", l()))
	require.Equal(t, 1.0, gauge(t, reg, "fulfillment_orders_in_progress", l("status", "paid")))

	state.Tick()
	require.NoError(t, state.SetDependencyMode(fulfillment.DependencyWarehouse, fulfillment.ModeSlow))
	m.Observe(state.Snapshot())
	require.Equal(t, 0.0, gauge(t, reg, "fulfillment_orders_in_progress", l("status", "paid")))
	require.Equal(t, 38.0, gauge(t, reg, "fulfillment_product_stock", l("sku", "sku-001")))
	require.Equal(t, 1.0, gauge(t, reg, "fulfillment_dependency_mode", l("dependency", "warehouse", "mode", "slow")))
	require.Equal(t, 0.0, gauge(t, reg, "fulfillment_dependency_mode", l("dependency", "warehouse", "mode", "healthy")))
}

func TestInstrumentHandler(t *testing.T) {
	m, reg := newMetrics(t)
	h := m.InstrumentHandler("probe", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	got := counter(t, reg, "inspected_http_requests_total", l("code", "418", "handler", "probe", "method", "get"))
	require.Equal(t, 1.0, got)
	count, err := testutil.GatherAndCount(reg, "inspected_http_request_duration_seconds")
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestMetricsPassLint(t *testing.T) {
	m, reg := newMetrics(t)
	state, err := fulfillment.NewState(fulfillment.StandardCatalog())
	require.NoError(t, err)
	m.Observe(state.Snapshot())
	m.Record([]fulfillment.Event{
		fulfillment.OrderPlaced{Order: "ord-000001", SKU: "sku-001", Quantity: 1, Channel: fulfillment.ChannelAPI},
		fulfillment.OrderStatusChanged{
			From: fulfillment.StatusPaid, To: fulfillment.StatusShipped, Reason: fulfillment.ReasonShipped, At: 2,
		},
		fulfillment.DependencyCalled{Dependency: fulfillment.DependencyWarehouse, Succeeded: true},
		fulfillment.ProductRestocked{SKU: "sku-003", From: 0, To: 10, At: 10},
	})
	m.InstrumentHandler("probe", http.NotFoundHandler()).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	problems, err := testutil.GatherAndLint(reg)

	require.NoError(t, err)
	require.Empty(t, problems)
}
