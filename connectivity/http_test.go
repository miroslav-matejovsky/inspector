package connectivity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/connectivity"
)

// jsonHandler answers every request with status and a JSON body.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// serve starts a test server and returns a reader for its root with a 1
// second timeout.
func serve(t *testing.T, h http.Handler) (*httptest.Server, *connectivity.HTTPReader) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	r, err := connectivity.NewHTTPReader(srv.URL, time.Second, srv.Client())
	require.NoError(t, err)
	return srv, r
}

func TestNewHTTPReaderRejectsInvalidConfig(t *testing.T) {
	tests := map[string]struct {
		base    string
		timeout time.Duration
		client  *http.Client
	}{
		"empty base":     {"", time.Second, http.DefaultClient},
		"no scheme":      {"localhost:8080", time.Second, http.DefaultClient},
		"ftp scheme":     {"ftp://example.test", time.Second, http.DefaultClient},
		"no host":        {"http://", time.Second, http.DefaultClient},
		"trailing slash": {"http://example.test/api/", time.Second, http.DefaultClient},
		"query":          {"http://example.test?x=1", time.Second, http.DefaultClient},
		"fragment":       {"http://example.test#f", time.Second, http.DefaultClient},
		"zero timeout":   {"http://example.test", 0, http.DefaultClient},
		"nil client":     {"http://example.test", time.Second, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := connectivity.NewHTTPReader(tc.base, tc.timeout, tc.client)

			require.Error(t, err)
		})
	}
}

func TestGetReadsDocument(t *testing.T) {
	var method, query, accept string
	mux := http.NewServeMux()
	mux.HandleFunc("/base/items", func(w http.ResponseWriter, r *http.Request) {
		method, query, accept = r.Method, r.URL.RawQuery, r.Header.Get("Accept")
		jsonHandler(http.StatusOK, `{"a":1}`)(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	reader, err := connectivity.NewHTTPReader(srv.URL+"/base", time.Second, srv.Client())
	require.NoError(t, err)

	doc, err := reader.Get(context.Background(), "/items?x=1")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, doc.Status)
	require.JSONEq(t, `{"a":1}`, string(doc.Body))
	require.Equal(t, srv.URL+"/base/items?x=1", doc.URL)
	require.Equal(t, http.MethodGet, method)
	require.Equal(t, "x=1", query)
	require.Equal(t, "application/json", accept)
}

func TestGetReturnsErrorStatusAsDocument(t *testing.T) {
	_, reader := serve(t, jsonHandler(http.StatusServiceUnavailable, `{"status":"down"}`))

	doc, err := reader.Get(context.Background(), "/ready")

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, doc.Status)
	require.JSONEq(t, `{"status":"down"}`, string(doc.Body))
}

func TestGetAcceptsJSONWithCharset(t *testing.T) {
	_, reader := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{}`))
	}))

	_, err := reader.Get(context.Background(), "/x")

	require.NoError(t, err)
}

func TestGetRejectsInvalidPath(t *testing.T) {
	var requests atomic.Int32
	_, reader := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		jsonHandler(http.StatusOK, `{}`)(w, r)
	}))

	for _, path := range []string{"", "items", "//other.test/x", "http://other.test/x"} {
		t.Run(path, func(t *testing.T) {
			_, err := reader.Get(context.Background(), path)

			require.Error(t, err)
		})
	}
	require.Zero(t, requests.Load())
}

func TestGetRejectsNonJSON(t *testing.T) {
	_, reader := serve(t, http.NotFoundHandler())

	_, err := reader.Get(context.Background(), "/missing")

	require.ErrorContains(t, err, "status 404")
	require.ErrorContains(t, err, "text/plain")
}

func TestGetRejectsOversizedBody(t *testing.T) {
	body := strings.Repeat(" ", connectivity.MaxDocumentBytes+1)
	_, reader := serve(t, jsonHandler(http.StatusOK, body))

	_, err := reader.Get(context.Background(), "/big")

	require.ErrorContains(t, err, "exceeds")
}

func TestGetTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	reader, err := connectivity.NewHTTPReader(srv.URL, 50*time.Millisecond, srv.Client())
	require.NoError(t, err)

	_, err = reader.Get(context.Background(), "/slow")

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetFailsWhenSourceIsDown(t *testing.T) {
	srv, reader := serve(t, jsonHandler(http.StatusOK, `{}`))
	srv.Close()

	_, err := reader.Get(context.Background(), "/x")

	require.Error(t, err)
}

func TestDocumentDecode(t *testing.T) {
	var v struct{ Name string }

	err := connectivity.Document{URL: "u", Body: []byte(`{"name":"x","extra":1}`)}.Decode(&v)

	require.NoError(t, err)
	require.Equal(t, "x", v.Name)

	err = connectivity.Document{URL: "u", Body: []byte(`{`)}.Decode(&v)

	require.ErrorContains(t, err, "decode u")
}
