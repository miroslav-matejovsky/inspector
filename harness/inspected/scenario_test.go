package inspected_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The scenarios inject a cause through the control API and check that it is
// visible consistently in the business API, the health checks and the metrics.

func setDependency(t *testing.T, h http.Handler, name, mode string) {
	t.Helper()
	rec := serve(t, h, http.MethodPut, prefix+"/sim/dependencies/"+name, fmt.Sprintf(`{"mode":%q}`, mode))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func advance(t *testing.T, h http.Handler, ticks int) {
	t.Helper()
	rec := serve(t, h, http.MethodPost, prefix+"/sim/advance", fmt.Sprintf(`{"ticks":%d}`, ticks))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func placeOrder(t *testing.T, h http.Handler, sku string, quantity int) {
	t.Helper()
	rec := serve(t, h, http.MethodPost, ordersPath, fmt.Sprintf(`{"sku":%q,"quantity":%d}`, sku, quantity))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func getOrder(t *testing.T, h http.Handler, id string) orderBody {
	t.Helper()
	rec := serve(t, h, http.MethodGet, ordersPath+"/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decode[orderBody](t, rec)
}

func ready(t *testing.T, h http.Handler) (int, healthBody) {
	t.Helper()
	rec := serve(t, h, http.MethodGet, prefix+"/health/ready", "")
	return rec.Code, decode[healthBody](t, rec)
}

func scrape(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := serve(t, h, http.MethodGet, prefix+"/metrics", "")
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func requireMetric(t *testing.T, metrics, line string) {
	t.Helper()
	require.Contains(t, metrics, "\n"+line+"\n")
}

func TestScenarioPaymentGatewayOutage(t *testing.T) {
	h := startApp(t, testConfig()).Handler()

	setDependency(t, h, "payment-gateway", "outage")
	placeOrder(t, h, "sku-001", 1)
	advance(t, h, 3)

	order := getOrder(t, h, "ord-000001")
	require.Equal(t, "failed", order.Status)
	require.Equal(t, "payment_gateway_unavailable", order.FailureReason)

	code, report := ready(t, h)
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.Equal(t, "down", report.Status)
	require.Equal(t, "down", report.Checks[0].Status)

	metrics := scrape(t, h)
	requireMetric(t, metrics, `fulfillment_dependency_calls_total{dependency="payment-gateway",result="failure"} 3`)
	requireMetric(t, metrics, `fulfillment_order_transitions_total{from="pending",reason="payment_gateway_unavailable",to="failed"} 1`)
	requireMetric(t, metrics, `fulfillment_dependency_up{dependency="payment-gateway"} 0`)
}

func TestScenarioSlowPaymentGateway(t *testing.T) {
	h := startApp(t, testConfig()).Handler()

	setDependency(t, h, "payment-gateway", "slow")
	placeOrder(t, h, "sku-001", 1)

	for _, step := range []struct {
		ticks  int
		status string
	}{{2, "pending"}, {1, "paid"}, {1, "shipped"}} {
		advance(t, h, step.ticks)
		require.Equal(t, step.status, getOrder(t, h, "ord-000001").Status)

		code, report := ready(t, h)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, "degraded", report.Status)
	}
}

func TestScenarioOutOfStockAndRestock(t *testing.T) {
	h := startApp(t, testConfig()).Handler()

	for range 11 {
		placeOrder(t, h, "sku-003", 1)
	}
	advance(t, h, 2)

	for i := 1; i <= 10; i++ {
		require.Equal(t, "shipped", getOrder(t, h, fmt.Sprintf("ord-%06d", i)).Status)
	}
	last := getOrder(t, h, "ord-000011")
	require.Equal(t, "failed", last.Status)
	require.Equal(t, "out_of_stock", last.FailureReason)

	rec := serve(t, h, http.MethodGet, productsPath+"/sku-003", "")
	require.Equal(t, 0, decode[productBody](t, rec).Stock)
	code, report := ready(t, h)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "degraded", report.Status)
	require.Equal(t, checkBody{Name: "inventory", Status: "degraded", Reason: "out of stock: sku-003"}, report.Checks[2])
	requireMetric(t, scrape(t, h), `fulfillment_product_stock{sku="sku-003"} 0`)

	advance(t, h, 8) // tick 10 restocks

	rec = serve(t, h, http.MethodGet, productsPath+"/sku-003", "")
	require.Equal(t, 10, decode[productBody](t, rec).Stock)
	_, report = ready(t, h)
	require.Equal(t, "up", report.Status)
	requireMetric(t, scrape(t, h), `fulfillment_product_restocks_total{sku="sku-003"} 1`)
}

func TestScenarioWarehouseOutage(t *testing.T) {
	h := startApp(t, testConfig()).Handler()

	setDependency(t, h, "warehouse", "outage")
	placeOrder(t, h, "sku-001", 1)
	advance(t, h, 4)

	order := getOrder(t, h, "ord-000001")
	require.Equal(t, "failed", order.Status)
	require.Equal(t, "warehouse_unavailable", order.FailureReason)
	var ticks []uint64
	for _, tr := range order.History {
		ticks = append(ticks, tr.AtTick)
	}
	require.Equal(t, []uint64{0, 1, 4}, ticks)
}

func TestScenarioSimulatedTrafficIsReproducible(t *testing.T) {
	cfg := testConfig()
	cfg.Seed = 7
	cfg.OrdersPerTick = 2

	run := func() (orders string, metrics string) {
		h := startApp(t, cfg).Handler()
		advance(t, h, 15)
		var lines []string
		for _, line := range strings.Split(scrape(t, h), "\n") {
			if strings.HasPrefix(line, "fulfillment_") {
				lines = append(lines, line)
			}
		}
		return serve(t, h, http.MethodGet, ordersPath, "").Body.String(), strings.Join(lines, "\n")
	}

	ordersA, metricsA := run()
	ordersB, metricsB := run()

	require.Equal(t, ordersA, ordersB)
	require.Equal(t, metricsA, metricsB)
	require.Equal(t, 30, strings.Count(ordersA, `"id":"ord-`))
}
