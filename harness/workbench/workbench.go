package workbench

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
)

//go:embed index.html
var indexHTML []byte

const shutdownTimeout = 5 * time.Second

// Handler serves the workbench page at "/" and delegates every path under
// inspected.PathPrefix+"/" to inspectedHandler, without stripping the prefix.
func Handler(inspectedHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.Handle(inspected.PathPrefix+"/", inspectedHandler)
	return mux
}

// Run validates cfg, starts the inspected app and the HTTP server with the
// inspected service mounted, and blocks until ctx is cancelled or one of
// them fails. Run is the supervisor of both:
// an unexpected stop of either one cancels the other and is returned as an
// error. A clean shutdown returns nil. Restart is not attempted; the process
// exits and a new start rebuilds the inspected state from the seed.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	app, err := inspected.New(cfg.Inspected)
	if err != nil {
		return fmt.Errorf("workbench: %w", err)
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("workbench: listen on %q: %w", cfg.Addr, err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	appDone := make(chan error, 1)
	go func() { appDone <- app.Run(runCtx) }()

	srv := &http.Server{
		Handler:           Handler(app.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	var errs []error
	appExited, serveExited := false, false
	select {
	case <-ctx.Done():
	case err := <-appDone:
		appExited = true
		if err == nil && ctx.Err() != nil {
			// ctx was cancelled and select picked this ready case first:
			// the app stopped because of the cancellation, a clean shutdown.
			break
		}
		if err == nil {
			err = errors.New("unexpected stop")
		}
		errs = append(errs, fmt.Errorf("workbench: inspected stopped: %w", err))
	case err := <-serveDone:
		serveExited = true
		errs = append(errs, fmt.Errorf("workbench: serve: %w", err))
	}

	// ctx may already be cancelled, so shutdown needs its own deadline.
	cancel()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("workbench: shutdown: %w", err))
	}

	// Wait for both goroutines: Run owns them.
	if !serveExited {
		if err := <-serveDone; !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, fmt.Errorf("workbench: serve: %w", err))
		}
	}
	if !appExited {
		if err := <-appDone; err != nil {
			errs = append(errs, fmt.Errorf("workbench: inspected: %w", err))
		}
	}
	return errors.Join(errs...)
}
