package inspected_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
)

const prefix = inspected.PathPrefix

func TestIndex(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/", "")

	require.Equal(t, http.StatusOK, rec.Code)
	body := decode[struct{ Links map[string]string }](t, rec)
	require.NotEmpty(t, body.Links)
	for name, link := range body.Links {
		require.Contains(t, link, prefix+"/", name)
	}
}

func TestIndexLinksResolve(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	body := decode[struct{ Links map[string]string }](t, serve(t, h, http.MethodGet, prefix+"/", ""))

	for name, link := range body.Links {
		rec := serve(t, h, http.MethodGet, link, "")
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", name, link)
	}
}

func TestLive(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/health/live", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"up"}`, rec.Body.String())
}

func TestLiveTimesOutWhenSimulationNotRunning(t *testing.T) {
	cfg := testConfig()
	cfg.RequestTimeout = time.Millisecond
	app, err := inspected.New(cfg) // Run is never called
	require.NoError(t, err)

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/health/live", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decode[struct{ Status, Reason string }](t, rec)
	require.Equal(t, "down", body.Status)
	require.Contains(t, body.Reason, "deadline exceeded")
}

func TestReady(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/health/ready", "")

	require.Equal(t, http.StatusOK, rec.Code)
	body := decode[healthBody](t, rec)
	require.Equal(t, "up", body.Status)
	require.Len(t, body.Checks, 3)
	require.Equal(t, "payment-gateway", body.Checks[0].Name)
	require.Equal(t, "warehouse", body.Checks[1].Name)
	require.Equal(t, "inventory", body.Checks[2].Name)
}

func TestReadyDownDuringOutage(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	rec := serve(t, h, http.MethodPut, prefix+"/sim/dependencies/payment-gateway", `{"mode":"outage"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = serve(t, h, http.MethodGet, prefix+"/health/ready", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decode[healthBody](t, rec)
	require.Equal(t, "down", body.Status)
	require.Equal(t, checkBody{
		Name: "payment-gateway", Status: "down", Reason: "payment-gateway is in outage: calls fail",
	}, body.Checks[0])
}

func TestReadyDegradedWhenSlow(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	rec := serve(t, h, http.MethodPut, prefix+"/sim/dependencies/warehouse", `{"mode":"slow"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = serve(t, h, http.MethodGet, prefix+"/health/ready", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "degraded", decode[healthBody](t, rec).Status)
}

func TestReadyReportsUnavailableSimulation(t *testing.T) {
	cfg := testConfig()
	cfg.RequestTimeout = time.Millisecond
	app, err := inspected.New(cfg) // Run is never called
	require.NoError(t, err)

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/health/ready", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decode[healthBody](t, rec)
	require.Equal(t, "down", body.Status)
	require.Len(t, body.Checks, 1)
	require.Equal(t, "simulation", body.Checks[0].Name)
	require.Contains(t, body.Checks[0].Reason, "deadline exceeded")
}

func TestMetrics(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/metrics", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
	body := rec.Body.String()
	require.Contains(t, body, "\nfulfillment_simulation_tick 0\n")
	require.Contains(t, body, "\ngo_goroutines ")
}

func TestMetricsFollowAdvance(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	rec := serve(t, h, http.MethodPost, prefix+"/sim/advance", `{"ticks":3}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = serve(t, h, http.MethodGet, prefix+"/metrics", "")

	require.Contains(t, rec.Body.String(), "\nfulfillment_simulation_tick 3\n")
}

func TestHTTPRequestsAreInstrumented(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	serve(t, h, http.MethodGet, prefix+"/health/live", "")

	rec := serve(t, h, http.MethodGet, prefix+"/metrics", "")

	require.Contains(t, rec.Body.String(),
		"\ninspected_http_requests_total{code=\"200\",handler=\"health_live\",method=\"get\"} 1\n")
}

func TestUnknownPathAndMethod(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	require.Equal(t, http.StatusNotFound, serve(t, h, http.MethodGet, prefix+"/nope", "").Code)
	require.Equal(t, http.StatusMethodNotAllowed, serve(t, h, http.MethodPost, prefix+"/health/live", "").Code)
}
