package workbench

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

//go:embed index.html
var indexHTML []byte

const shutdownTimeout = 5 * time.Second

// Handler returns the workbench HTTP handler. It serves the page at "/" only.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	return mux
}

// Run serves the workbench on addr until ctx is cancelled, then shuts down
// gracefully. addr must not be empty. It returns nil on a clean shutdown.
func Run(ctx context.Context, addr string) error {
	if addr == "" {
		return errors.New("workbench: listen address is required")
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("workbench: listen on %q: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("workbench: serve: %w", err)
	case <-ctx.Done():
	}

	// ctx is already cancelled, so shutdown needs its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("workbench: shutdown: %w", err)
	}
	return nil
}
