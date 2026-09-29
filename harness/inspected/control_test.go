package inspected_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimStatus(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/sim", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, simBody{
		NowTick: 0, ClockRunning: true, Seed: 1, OrdersPerTick: 0, TickInterval: "1h0m0s",
		Dependencies: []dependencyBody{{"payment-gateway", "healthy"}, {"warehouse", "healthy"}},
	}, decode[simBody](t, rec))
}

func TestSimClock(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	rec := serve(t, h, http.MethodPut, prefix+"/sim/clock", `{"running":false}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, decode[simBody](t, rec).ClockRunning)

	rec = serve(t, h, http.MethodPut, prefix+"/sim/clock", `{}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request", decode[errorBody](t, rec).Error.Code)
}

func TestSimAdvance(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	rec := serve(t, h, http.MethodPost, prefix+"/sim/advance", `{"ticks":5}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, uint64(5), decode[simBody](t, rec).NowTick)

	for _, body := range []string{`{"ticks":0}`, `{"ticks":1001}`} {
		rec = serve(t, h, http.MethodPost, prefix+"/sim/advance", body)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, body)
		require.Equal(t, "invalid_advance", decode[errorBody](t, rec).Error.Code, body)
	}

	for _, body := range []string{`{`, `{"ticks":1,"x":1}`, ``, `{"ticks":1} x`} {
		rec = serve(t, h, http.MethodPost, prefix+"/sim/advance", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		require.Equal(t, "invalid_request", decode[errorBody](t, rec).Error.Code, body)
	}
}

func TestSimDependency(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	rec := serve(t, h, http.MethodPut, prefix+"/sim/dependencies/warehouse", `{"mode":"slow"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, dependencyBody{Name: "warehouse", Mode: "slow"}, decode[dependencyBody](t, rec))

	rec = serve(t, h, http.MethodPut, prefix+"/sim/dependencies/unknown", `{"mode":"slow"}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "unknown_dependency", decode[errorBody](t, rec).Error.Code)

	rec = serve(t, h, http.MethodPut, prefix+"/sim/dependencies/warehouse", `{"mode":"down"}`)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "invalid_dependency_mode", decode[errorBody](t, rec).Error.Code)
}
