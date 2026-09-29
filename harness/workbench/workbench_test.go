package workbench_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

func TestHandlerServesTwoPanels(t *testing.T) {
	srv := httptest.NewServer(workbench.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `id="inspected"`)
	require.Contains(t, string(body), `id="inspector"`)
}

func TestHandlerUnknownPathIsNotFound(t *testing.T) {
	srv := httptest.NewServer(workbench.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/missing")
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRunRejectsEmptyAddr(t *testing.T) {
	err := workbench.Run(context.Background(), "")
	require.Error(t, err)
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- workbench.Run(ctx, "127.0.0.1:0") }()

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
