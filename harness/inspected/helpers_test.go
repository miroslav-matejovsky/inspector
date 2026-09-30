package inspected_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
)

// testConfig returns a config whose clock never fires during a test. Time
// moves only through POST /inspected/sim/advance.
func testConfig() inspected.Config {
	return inspected.Config{Seed: 1, OrdersPerTick: 0, TickInterval: time.Hour, RequestTimeout: time.Second}
}

// startApp creates the app and runs it. On cleanup it cancels Run and
// requires it to return nil. It returns after the simulation accepts requests,
// so the first metrics scrape already sees the initial state.
func startApp(t *testing.T, cfg inspected.Config) *inspected.App {
	t.Helper()
	app, err := inspected.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})

	rec := serve(t, app.Handler(), http.MethodGet, inspected.PathPrefix+"/sim", "")
	require.Equal(t, http.StatusOK, rec.Code)
	return app
}

// serve runs one request synchronously through h.
func serve(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decode parses the JSON response body into T.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}

type errorBody struct {
	Error struct {
		Code    string
		Message string
	}
}

type checkBody struct {
	Name   string
	Status string
	Reason string
}

type healthBody struct {
	Status string
	Checks []checkBody
}

type dependencyBody struct {
	Name string
	Mode string
}

type simBody struct {
	NowTick       uint64 `json:"now_tick"`
	ClockRunning  bool   `json:"clock_running"`
	Seed          uint64
	OrdersPerTick int    `json:"orders_per_tick"`
	TickInterval  string `json:"tick_interval"`
	Dependencies  []dependencyBody
}
