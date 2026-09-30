package source_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/source"
)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

var discard = slog.New(slog.DiscardHandler)

// server answers /a with 200 text and /b with 503 JSON, plus extra handlers.
func server(t *testing.T, extra map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "alpha body")
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"s":"down"}`)
	})
	for path, h := range extra {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func validConfig(t *testing.T, srv *httptest.Server) source.Config {
	t.Helper()
	return source.Config{
		Targets: []source.Target{
			{Name: "alpha", URL: srv.URL + "/a"},
			{Name: "beta", URL: srv.URL + "/b"},
		},
		Interval:     time.Hour,
		Timeout:      time.Second,
		Retention:    time.Hour,
		MaxBodyBytes: 1024,
		DatabasePath: filepath.Join(t.TempDir(), "s.db"),
	}
}

// clock is a fake clock the test sets between calls. Test handlers may move
// it while a read runs, so access is guarded.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// open opens a Source with a fake clock at t0 and closes it when the test ends.
func open(t *testing.T, cfg source.Config, logger *slog.Logger) (*source.Source, *clock) {
	t.Helper()
	s, err := source.Open(context.Background(), cfg, http.DefaultClient, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	c := &clock{now: t0}
	source.SetClock(s, c.Now)
	return s, c
}

func summary(t *testing.T, s *source.Source) map[string]source.TargetSummary {
	t.Helper()
	list, err := s.Summary(context.Background())
	require.NoError(t, err)
	out := map[string]source.TargetSummary{}
	for _, ts := range list {
		out[ts.Target.Name] = ts
	}
	return out
}

func bufferLogger(buf *bytes.Buffer) *slog.Logger {
	dropTime := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{ReplaceAttr: dropTime}))
}

func TestOpenRejectsArguments(t *testing.T) {
	srv := server(t, nil)
	invalid := validConfig(t, srv)
	invalid.Targets = nil
	tests := map[string]struct {
		cfg    source.Config
		client *http.Client
		logger *slog.Logger
	}{
		"invalid config": {invalid, http.DefaultClient, discard},
		"nil client":     {validConfig(t, srv), nil, discard},
		"nil logger":     {validConfig(t, srv), http.DefaultClient, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := source.Open(context.Background(), tc.cfg, tc.client, tc.logger)

			require.Error(t, err)
			require.NoFileExists(t, tc.cfg.DatabasePath)
		})
	}
}

func TestSummaryBeforeCollect(t *testing.T) {
	cfg := validConfig(t, server(t, nil))
	cfg.Targets[0], cfg.Targets[1] = cfg.Targets[1], cfg.Targets[0]
	s, _ := open(t, cfg, discard)

	got, err := s.Summary(context.Background())

	require.NoError(t, err)
	require.Equal(t, []source.TargetSummary{
		{Target: cfg.Targets[0]},
		{Target: cfg.Targets[1]},
	}, got)
	require.Equal(t, "beta", got[0].Target.Name)
}

func TestCollectStoresSignals(t *testing.T) {
	srv := server(t, nil)
	s, _ := open(t, validConfig(t, srv), discard)

	require.NoError(t, s.Collect(context.Background()))

	got := summary(t, s)
	require.Equal(t, int64(1), got["alpha"].Signals)
	require.Equal(t, &source.Signal{
		Target:      "alpha",
		URL:         srv.URL + "/a",
		ObservedAt:  t0,
		StatusCode:  200,
		ContentType: "text/plain; charset=utf-8",
		Body:        []byte("alpha body"),
	}, got["alpha"].Latest)
	beta := got["beta"].Latest
	require.Equal(t, 503, beta.StatusCode)
	require.Equal(t, `{"s":"down"}`, string(beta.Body))
	require.Empty(t, beta.Error)
}

func TestCollectMeasuresDuration(t *testing.T) {
	var c *clock
	srv := server(t, map[string]http.HandlerFunc{"/tick": func(http.ResponseWriter, *http.Request) {
		c.Set(c.Now().Add(1500 * time.Microsecond))
	}})
	cfg := validConfig(t, srv)
	cfg.Targets = []source.Target{{Name: "alpha", URL: srv.URL + "/tick"}}
	s, c := open(t, cfg, discard)

	require.NoError(t, s.Collect(context.Background()))

	latest := summary(t, s)["alpha"].Latest
	require.Equal(t, 1500*time.Microsecond, latest.Duration)
	require.Equal(t, t0, latest.ObservedAt)
}

func TestCollectUsesGet(t *testing.T) {
	var method string
	var bodyLen int
	srv := server(t, map[string]http.HandlerFunc{"/get": func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, bodyLen = r.Method, len(b)
	}})
	cfg := validConfig(t, srv)
	cfg.Targets = []source.Target{{Name: "alpha", URL: srv.URL + "/get"}}
	s, _ := open(t, cfg, discard)

	require.NoError(t, s.Collect(context.Background()))

	require.Equal(t, http.MethodGet, method)
	require.Equal(t, 0, bodyLen)
}

func TestCollectStoresFailedRead(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	cfg := validConfig(t, server(t, nil))
	cfg.Targets = []source.Target{{Name: "alpha", URL: closed.URL + "/a"}}
	s, _ := open(t, cfg, discard)

	require.NoError(t, s.Collect(context.Background()))

	latest := summary(t, s)["alpha"].Latest
	require.Equal(t, 0, latest.StatusCode)
	require.Nil(t, latest.Body)
	require.NotEmpty(t, latest.Error)
}

func TestCollectRejectsLargeBody(t *testing.T) {
	srv := server(t, map[string]http.HandlerFunc{
		"/ten":    func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", 10)) },
		"/eleven": func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", 11)) },
	})
	cfg := validConfig(t, srv)
	cfg.MaxBodyBytes = 10
	cfg.Targets = []source.Target{{Name: "ten", URL: srv.URL + "/ten"}, {Name: "eleven", URL: srv.URL + "/eleven"}}
	s, _ := open(t, cfg, discard)

	require.NoError(t, s.Collect(context.Background()))

	got := summary(t, s)
	require.Len(t, got["ten"].Latest.Body, 10)
	require.Empty(t, got["ten"].Latest.Error)
	eleven := got["eleven"].Latest
	require.Equal(t, 200, eleven.StatusCode)
	require.Nil(t, eleven.Body)
	require.Equal(t, "body exceeds 10 bytes", eleven.Error)
}

func TestCollectTimesOut(t *testing.T) {
	srv := server(t, map[string]http.HandlerFunc{"/slow": func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}})
	cfg := validConfig(t, srv)
	cfg.Timeout = 50 * time.Millisecond
	cfg.Targets = []source.Target{{Name: "alpha", URL: srv.URL + "/slow"}}
	s, _ := open(t, cfg, discard)

	require.NoError(t, s.Collect(context.Background()))

	require.Contains(t, summary(t, s)["alpha"].Latest.Error, "context deadline exceeded")
}

func TestCollectAppliesRetention(t *testing.T) {
	cfg := validConfig(t, server(t, nil))
	cfg.Retention = time.Minute
	s, c := open(t, cfg, discard)

	for _, at := range []time.Duration{0, 30 * time.Second, 90 * time.Second} {
		c.Set(t0.Add(at))
		require.NoError(t, s.Collect(context.Background()))
	}

	alpha := summary(t, s)["alpha"]
	require.Equal(t, int64(2), alpha.Signals)
	require.Equal(t, t0.Add(90*time.Second), alpha.Latest.ObservedAt)
}

func TestCollectLogsTransitionsOnce(t *testing.T) {
	var fail atomic.Bool
	srv := server(t, map[string]http.HandlerFunc{"/toggle": func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			_, _ = io.WriteString(w, "xx")
		}
	}})
	cfg := validConfig(t, srv)
	cfg.MaxBodyBytes = 1
	cfg.Targets = []source.Target{{Name: "alpha", URL: srv.URL + "/toggle"}}
	var buf bytes.Buffer
	s, _ := open(t, cfg, bufferLogger(&buf))

	for _, failing := range []bool{true, true, false, false} {
		fail.Store(failing)
		require.NoError(t, s.Collect(context.Background()))
	}

	log := buf.String()
	require.Equal(t, 1, strings.Count(log, `msg="source target failing"`), log)
	require.Equal(t, 1, strings.Count(log, `msg="source target recovered"`), log)
	require.Less(t, strings.Index(log, "failing"), strings.Index(log, "recovered"))
}

func TestSummaryIgnoresUnknownTargets(t *testing.T) {
	ctx := context.Background()
	cfg := validConfig(t, server(t, nil))
	cfg.Targets = cfg.Targets[:1]
	first, err := source.Open(ctx, cfg, http.DefaultClient, discard)
	require.NoError(t, err)
	require.NoError(t, first.Collect(ctx))
	require.NoError(t, first.Close())
	cfg.Targets = []source.Target{{Name: "beta", URL: "http://example.test/b"}}
	second, _ := open(t, cfg, discard)

	got, err := second.Summary(ctx)

	require.NoError(t, err)
	require.Equal(t, []source.TargetSummary{{Target: cfg.Targets[0]}}, got)
}

func TestRunStopsWhenContextEnds(t *testing.T) {
	var buf bytes.Buffer
	s, _ := open(t, validConfig(t, server(t, nil)), bufferLogger(&buf))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, s.Run(ctx))

	// Reads cut short by the end of ctx are not failures of the targets.
	require.NotContains(t, buf.String(), "source target failing")
	require.Equal(t, int64(0), summary(t, s)["alpha"].Signals)
}

func TestRunReturnsStoreError(t *testing.T) {
	var buf bytes.Buffer
	s, err := source.Open(context.Background(), validConfig(t, server(t, nil)), http.DefaultClient, bufferLogger(&buf))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	err = s.Run(context.Background())

	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "source:"), err.Error())
	require.Contains(t, buf.String(), `msg="source started"`)
}
