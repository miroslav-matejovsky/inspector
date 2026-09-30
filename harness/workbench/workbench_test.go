package workbench_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
	"github.com/miroslav-matejovsky/inspector/source"
	"github.com/miroslav-matejovsky/inspector/view"
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

// page gets "/" from a workbench handler of a running inspected app that
// shows summaries.
func page(t *testing.T, summaries ...source.TargetSummary) string {
	t.Helper()
	rec := get(workbench.Handler(startInspected(t).Handler(), stubSignals{summaries: summaries}, discard), "/")
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func withLatest(name string, count int64, sig source.Signal) source.TargetSummary {
	sig.Target = name
	return source.TargetSummary{Target: source.Target{Name: name, URL: "http://example.test/" + name}, Signals: count, Latest: &sig}
}

func TestHandlerServesWorkbenchPanel(t *testing.T) {
	rec := get(workbench.Handler(startInspected(t).Handler(), stubSignals{}, discard), "/")

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

func TestPageShowsRawView(t *testing.T) {
	summaries := []source.TargetSummary{
		withLatest("alpha", 3, source.Signal{
			ObservedAt: t0, StatusCode: 200, ContentType: "application/json", Body: []byte(`{"status":"up"}`),
		}),
		{Target: source.Target{Name: "beta", URL: "http://example.test/beta"}},
	}
	want, err := view.Raw(summaries)
	require.NoError(t, err)

	body := page(t, summaries...)

	require.Contains(t, body, string(want))
	require.Contains(t, body, string(view.Styles))
}

func TestPageEscapesSignals(t *testing.T) {
	body := page(t, withLatest("alpha", 1, source.Signal{ObservedAt: t0, StatusCode: 200, Body: []byte("<script>alert(1)</script>")}))

	require.Contains(t, body, "&lt;script&gt;")
	require.NotContains(t, body, "<script>alert")
}

func TestPageRefreshes(t *testing.T) {
	body := page(t)

	require.Contains(t, body, `<body data-refresh-seconds="2">`)
	require.Contains(t, body, `<span id="refresh-status" class="error"></span>`)
	require.Contains(t, body, `<noscript><meta http-equiv="refresh" content="2"></noscript>`)
}

func TestPageShowsSummaryError(t *testing.T) {
	var buf bytes.Buffer
	h := workbench.Handler(startInspected(t).Handler(), stubSignals{err: errors.New("boom")}, slog.New(slog.NewTextHandler(&buf, nil)))

	rec := get(h, "/")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "signals unavailable: boom")
	require.NotContains(t, rec.Body.String(), `class="view-raw"`)
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
	require.Regexp(t, `<tr class="target ok">.*>live</a></td><td class="num">1</td>.*>200</td>`, rec.Body.String())
	require.Regexp(t, `<tr class="target ok">.*>ready</a></td><td class="num">1</td>.*>200</td>`, rec.Body.String())
	require.Contains(t, rec.Body.String(), `<span class="key">&#34;status&#34;</span>`)
}

func postMode(h http.Handler, dependency, mode string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/controls/dependencies/"+dependency, strings.NewReader(url.Values{"mode": {mode}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// controls returns the form of dependency in body.
func controls(t *testing.T, body, dependency string) string {
	t.Helper()
	form := `<form method="post" action="/controls/dependencies/` + dependency + `">`
	start := strings.Index(body, form)
	require.GreaterOrEqual(t, start, 0, "no form for %q", dependency)
	return body[start : start+strings.Index(body[start:], "</form>")]
}

func TestPageHasDependencyControls(t *testing.T) {
	body := page(t)

	for _, dep := range []string{"payment-gateway", "warehouse"} {
		form := controls(t, body, dep)
		for _, mode := range []string{"healthy", "slow", "outage"} {
			require.Contains(t, form, `<button name="mode" value="`+mode+`"`)
		}
	}
}

func TestPageMarksActiveMode(t *testing.T) {
	h := workbench.Handler(startInspected(t).Handler(), stubSignals{}, discard)
	require.Equal(t, http.StatusSeeOther, postMode(h, "payment-gateway", "outage").Code)

	rec := get(h, "/")

	require.Equal(t, http.StatusOK, rec.Code)
	payment := controls(t, rec.Body.String(), "payment-gateway")
	require.Contains(t, payment, `value="outage" class="mode-outage active" aria-pressed="true"`)
	require.Contains(t, payment, `value="healthy" class="mode-healthy" aria-pressed="false"`)
	require.Equal(t, 1, strings.Count(payment, `aria-pressed="true"`))
	warehouse := controls(t, rec.Body.String(), "warehouse")
	require.Contains(t, warehouse, `value="healthy" class="mode-healthy active" aria-pressed="true"`)
	require.Equal(t, 1, strings.Count(warehouse, `aria-pressed="true"`))
}

func TestPageShowsControlsError(t *testing.T) {
	var buf bytes.Buffer
	unavailable := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "simulation stopped", http.StatusServiceUnavailable)
	})
	h := workbench.Handler(unavailable, stubSignals{}, slog.New(slog.NewTextHandler(&buf, nil)))

	rec := get(h, "/")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "dependency modes unavailable")
	require.Contains(t, rec.Body.String(), "simulation stopped")
	require.Contains(t, rec.Body.String(), `class="view-raw"`)
	require.Contains(t, buf.String(), `msg="workbench: read dependency modes"`)
}

func TestDependencyControlSetsMode(t *testing.T) {
	app := startInspected(t)
	h := workbench.Handler(app.Handler(), stubSignals{}, discard)

	rec := postMode(h, "payment-gateway", "outage")

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/", rec.Header().Get("Location"))
	dep := get(h, inspected.PathPrefix+"/api/dependencies/payment-gateway")
	require.Equal(t, http.StatusOK, dep.Code)
	require.Contains(t, dep.Body.String(), `"mode":"outage"`)
}

func TestDependencyControlShowsInspectedError(t *testing.T) {
	app := startInspected(t)
	h := workbench.Handler(app.Handler(), stubSignals{}, discard)

	rec := postMode(h, "payment-gateway", "bogus")

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Contains(t, rec.Body.String(), "set payment-gateway to bogus")
	require.Contains(t, rec.Body.String(), "invalid_dependency_mode")
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
