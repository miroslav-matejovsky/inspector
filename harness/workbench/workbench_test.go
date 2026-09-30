package workbench_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
	"github.com/miroslav-matejovsky/inspector/source"
)

var discard = slog.New(slog.DiscardHandler)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

// stubHandler records the paths it is asked to serve.
type stubHandler struct{ paths []string }

func (s *stubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.paths = append(s.paths, r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

// stubSignals returns fixed summaries or an error.
type stubSignals struct {
	summaries []source.TargetSummary
	err       error
}

func (s stubSignals) Summary(context.Context) ([]source.TargetSummary, error) {
	return s.summaries, s.err
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func inspectedConfig() inspected.Config {
	return inspected.Config{Seed: 1, OrdersPerTick: 1, TickInterval: time.Hour, RequestTimeout: time.Second}
}

func validConfig(t *testing.T) workbench.Config {
	t.Helper()
	return workbench.Config{
		Addr: "127.0.0.1:0",
		// Run does not use LogDir, so no directory is created.
		LogDir:    "logs",
		Inspected: inspectedConfig(),
		Source: workbench.SourceConfig{
			DatabasePath: filepath.Join(t.TempDir(), "source.db"),
			Interval:     time.Hour,
			Timeout:      time.Second,
			Retention:    time.Hour,
			MaxBodyBytes: 1 << 20,
			Targets:      []workbench.SourceTarget{{Name: "live", Path: "/inspected/health/live"}},
		},
	}
}

// startInspected creates the inspected app and runs it until the test ends.
// Its clock never fires during a test.
func startInspected(t *testing.T) *inspected.App {
	t.Helper()
	app, err := inspected.New(inspectedConfig())
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

// page gets "/" from a workbench handler that shows summaries.
func page(t *testing.T, summaries ...source.TargetSummary) string {
	t.Helper()
	rec := get(workbench.Handler(&stubHandler{}, stubSignals{summaries: summaries}, discard), "/")
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func withLatest(name string, count int64, sig source.Signal) source.TargetSummary {
	sig.Target = name
	return source.TargetSummary{Target: source.Target{Name: name, URL: "http://example.test/" + name}, Signals: count, Latest: &sig}
}

func TestHandlerServesWorkbenchPanel(t *testing.T) {
	rec := get(workbench.Handler(&stubHandler{}, stubSignals{}, discard), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, rec.Body.String(), `id="workbench"`)
}

func TestHandlerMountsInspected(t *testing.T) {
	inspectedStub := &stubHandler{}

	rec := get(workbench.Handler(inspectedStub, stubSignals{}, discard), inspected.PathPrefix+"/api/products")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, []string{inspected.PathPrefix + "/api/products"}, inspectedStub.paths)
}

func TestHandlerUnknownPathIsNotFound(t *testing.T) {
	inspectedStub := &stubHandler{}

	rec := get(workbench.Handler(inspectedStub, stubSignals{}, discard), "/missing")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Empty(t, inspectedStub.paths)
}

func TestWorkbenchServesInspected(t *testing.T) {
	app := startInspected(t)

	rec := get(workbench.Handler(app.Handler(), stubSignals{}, discard), inspected.PathPrefix+"/health/live")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"up"}`, rec.Body.String())
}

func TestPageShowsSignals(t *testing.T) {
	body := page(t,
		withLatest("alpha", 3, source.Signal{
			ObservedAt: t0, StatusCode: 200, Duration: 1500 * time.Microsecond, Body: []byte("abc"),
		}),
		source.TargetSummary{Target: source.Target{Name: "beta", URL: "http://example.test/beta"}},
	)

	require.Regexp(t, `alpha\s+3\s+2026-09-30T01:02:03Z\s+200\s+1\.5ms\s+3\s+-`, body)
	require.Regexp(t, `beta\s+0\s+-\s+-\s+-\s+-\s+-`, body)
	require.Contains(t, body, "--- alpha: 3 bytes ---")
}

func TestPageShowsFailedRead(t *testing.T) {
	body := page(t, withLatest("alpha", 1, source.Signal{ObservedAt: t0, Error: "connection refused"}))

	require.Regexp(t, `alpha\s+1\s+\S+\s+-\s+\S+\s+0\s+connection refused`, body)
}

func TestPageTruncatesPreview(t *testing.T) {
	body := page(t, withLatest("alpha", 1, source.Signal{ObservedAt: t0, StatusCode: 200, Body: bytes.Repeat([]byte("x"), 600)}))

	require.Contains(t, body, "--- alpha: first 512 of 600 bytes ---")
	require.Contains(t, body, strings.Repeat("x", 512))
	require.NotContains(t, body, strings.Repeat("x", 513))
}

func TestPageEscapesSignals(t *testing.T) {
	body := page(t, withLatest("alpha", 1, source.Signal{ObservedAt: t0, StatusCode: 200, Body: []byte("<script>alert(1)</script>")}))

	require.Contains(t, body, "&lt;script&gt;")
	require.NotContains(t, body, "<script>alert")
}

func TestPageRefreshes(t *testing.T) {
	require.Contains(t, page(t), `<meta http-equiv="refresh" content="2">`)
}

func TestPageShowsSummaryError(t *testing.T) {
	var buf bytes.Buffer
	h := workbench.Handler(&stubHandler{}, stubSignals{err: errors.New("boom")}, slog.New(slog.NewTextHandler(&buf, nil)))

	rec := get(h, "/")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "signals unavailable: boom")
	require.Contains(t, buf.String(), `msg="workbench: read signals"`)
}

func TestPageShowsCollectedSignals(t *testing.T) {
	ctx := context.Background()
	app := startInspected(t)
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	src, err := source.Open(ctx, source.Config{
		Targets: []source.Target{
			{Name: "live", URL: srv.URL + inspected.PathPrefix + "/health/live"},
			{Name: "ready", URL: srv.URL + inspected.PathPrefix + "/health/ready"},
		},
		Interval:     time.Hour,
		Timeout:      time.Second,
		Retention:    time.Hour,
		MaxBodyBytes: 1 << 20,
		DatabasePath: filepath.Join(t.TempDir(), "source.db"),
	}, srv.Client(), discard)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, src.Close()) })
	require.NoError(t, src.Collect(ctx))

	rec := get(workbench.Handler(app.Handler(), src, discard), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Regexp(t, `live\s+1\s+\S+\s+200`, rec.Body.String())
	require.Regexp(t, `ready\s+1\s+\S+\s+200`, rec.Body.String())
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	err := workbench.Run(context.Background(), workbench.Config{}, discard)

	require.Error(t, err)
}

func TestRunRejectsNilLogger(t *testing.T) {
	err := workbench.Run(context.Background(), validConfig(t), nil)

	require.ErrorContains(t, err, "logger is required")
}

func TestRunLogsStartAndStop(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := workbench.Run(ctx, validConfig(t), slog.New(slog.NewTextHandler(&buf, nil)))

	require.NoError(t, err)
	log := buf.String()
	require.Regexp(t, `msg="workbench started" addr=127\.0\.0\.1:\d+`, log)
	require.Contains(t, log, `msg="workbench stopped"`)
	require.Less(t, strings.Index(log, "workbench started"), strings.Index(log, "workbench stopped"))
	require.NotContains(t, log, "error=")
}

func TestRunCreatesDatabase(t *testing.T) {
	cfg := validConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, workbench.Run(ctx, cfg, discard))

	require.FileExists(t, cfg.Source.DatabasePath)
}

func TestRunFailsWhenSourceCannotOpen(t *testing.T) {
	cfg := validConfig(t)
	require.NoError(t, os.WriteFile(cfg.Source.DatabasePath, []byte("not a database"), 0o644))

	err := workbench.Run(context.Background(), cfg, discard)

	require.ErrorContains(t, err, "workbench: source:")
}

func TestRunFailsWhenAddressIsTaken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ln.Close()) })
	cfg := validConfig(t)
	cfg.Addr = ln.Addr().String()

	err = workbench.Run(context.Background(), cfg, discard)

	require.ErrorContains(t, err, "listen")
}

func TestRunStopsOnContextCancel(t *testing.T) {
	cfg := validConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- workbench.Run(ctx, cfg, discard) }()

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
