package workbench_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

// stubHandler records the paths it is asked to serve.
type stubHandler struct{ paths []string }

func (s *stubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.paths = append(s.paths, r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func validConfig() workbench.Config {
	return workbench.Config{
		Addr: "127.0.0.1:0",
		Inspected: inspected.Config{
			Seed: 1, OrdersPerTick: 1, TickInterval: time.Hour, RequestTimeout: time.Second,
		},
		InspectorSourceTimeout: time.Second,
	}
}

// startInspected creates the inspected app and runs it until the test ends.
// Its clock never fires during a test.
func startInspected(t *testing.T) *inspected.App {
	t.Helper()
	app, err := inspected.New(validConfig().Inspected)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return app
}

func TestHandlerServesTwoPanels(t *testing.T) {
	rec := get(workbench.Handler(&stubHandler{}, &stubHandler{}), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, rec.Body.String(), `id="inspected"`)
	require.Contains(t, rec.Body.String(), `id="inspector"`)
}

func TestHandlerMountsInspected(t *testing.T) {
	inspectedStub, inspectorStub := &stubHandler{}, &stubHandler{}

	rec := get(workbench.Handler(inspectedStub, inspectorStub), inspected.PathPrefix+"/api/products")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, []string{inspected.PathPrefix + "/api/products"}, inspectedStub.paths)
	require.Empty(t, inspectorStub.paths)
}

func TestHandlerMountsInspector(t *testing.T) {
	inspectedStub, inspectorStub := &stubHandler{}, &stubHandler{}

	rec := get(workbench.Handler(inspectedStub, inspectorStub), workbench.InspectorPathPrefix+"/entities")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, []string{workbench.InspectorPathPrefix + "/entities"}, inspectorStub.paths)
	require.Empty(t, inspectedStub.paths)
}

func TestHandlerUnknownPathIsNotFound(t *testing.T) {
	inspectedStub, inspectorStub := &stubHandler{}, &stubHandler{}

	rec := get(workbench.Handler(inspectedStub, inspectorStub), "/missing")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Empty(t, inspectedStub.paths)
	require.Empty(t, inspectorStub.paths)
}

func TestWorkbenchServesInspected(t *testing.T) {
	app := startInspected(t)

	rec := get(workbench.Handler(app.Handler(), &stubHandler{}), inspected.PathPrefix+"/health/live")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"up"}`, rec.Body.String())
}

func TestInspectorHandlerObservesInspected(t *testing.T) {
	app := startInspected(t)
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	h, err := workbench.InspectorHandler(srv.URL+inspected.PathPrefix, time.Second)
	require.NoError(t, err)

	rec := get(workbench.Handler(app.Handler(), h), workbench.InspectorPathPrefix+"/")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	type kind struct {
		Kind  string
		Count int
	}
	var overview struct{ Kinds []kind }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &overview))
	require.Equal(t, []kind{{"service", 1}, {"health_check", 3}, {"product", 5}}, overview.Kinds)
}

func TestInspectorHandlerRejectsInvalidConfig(t *testing.T) {
	_, err := workbench.InspectorHandler("", time.Second)
	require.Error(t, err)

	_, err = workbench.InspectorHandler("http://127.0.0.1:1/inspected", 0)
	require.Error(t, err)
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	err := workbench.Run(context.Background(), workbench.Config{})

	require.Error(t, err)
}

func TestRunFailsWhenAddressIsTaken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ln.Close()) })
	cfg := validConfig()
	cfg.Addr = ln.Addr().String()

	err = workbench.Run(context.Background(), cfg)

	require.ErrorContains(t, err, "listen")
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- workbench.Run(ctx, validConfig()) }()

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
