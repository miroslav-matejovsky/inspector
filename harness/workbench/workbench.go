package workbench

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/source"
	"github.com/miroslav-matejovsky/inspector/view"
	"github.com/miroslav-matejovsky/inspector/view/chart"
	"github.com/miroslav-matejovsky/inspector/view/dashboard"
	"github.com/miroslav-matejovsky/inspector/view/explorer"
	"github.com/miroslav-matejovsky/inspector/view/raw"
)

//go:embed index.html
var indexHTML string

// page is the template of every workbench page; the view differs per page.
var page = template.Must(template.New("index").Parse(indexHTML))

// pageRefreshSeconds is how often the page reloads itself.
const pageRefreshSeconds = 2

const shutdownTimeout = 5 * time.Second

// Signals returns what the Source of the workbench collected, one summary per target.
type Signals interface {
	Summary(ctx context.Context) ([]source.TargetSummary, error)
}

// Paths of the pages, in tab order.
const (
	pathDashboard = "/"
	pathModel     = "/model"
	pathRaw       = "/raw"
)

// tabs are the pages of the workbench, in tab order.
var tabs = []tab{{Path: pathDashboard, Label: "Dashboard"}, {Path: pathModel, Label: "Model"}, {Path: pathRaw, Label: "Raw"}}

// tab is one page in the panel header. Active marks the page shown.
type tab struct {
	Path, Label string
	Active      bool
}

// pageStyles is the CSS of every view, in the order a page includes it.
var pageStyles = []template.CSS{view.Styles, raw.Styles, explorer.Styles, chart.Styles, dashboard.Styles}

// pageData is the input of the page template.
type pageData struct {
	RefreshSeconds int
	Tabs           []tab  // copy of tabs, Active set for the page shown
	Return         string // r.URL.RequestURI(): the page to return to after a control
	Controls       []dependencyControl
	ControlsError  string
	Styles         []template.CSS
	View           template.HTML
	Error          string
}

// pageView is what a page shows below the header.
type pageView struct {
	View   template.HTML
	Status int    // HTTP status of the page
	Error  string // shown instead of View when not empty
}

// renderView renders the view of a page from the summaries of the Source.
// An error means the view could not be rendered.
type renderView func(r *http.Request, summaries []source.TargetSummary) (pageView, error)

func renderDashboard(_ *http.Request, s []source.TargetSummary) (pageView, error) {
	v, err := dashboard.Render(model.Build(s, inspectedInterpreter), pathModel)
	return pageView{View: v, Status: http.StatusOK}, err
}

// renderModel shows the explorer index, or the entity of the query parameter
// entity. An entity that is not in the model is a 404 page, not an error:
// an order may be pruned between two refreshes.
func renderModel(r *http.Request, s []source.TargetSummary) (pageView, error) {
	m := model.Build(s, inspectedInterpreter)
	id := r.URL.Query().Get("entity")
	if id == "" {
		v, err := explorer.Index(m, pathModel)
		return pageView{View: v, Status: http.StatusOK}, err
	}
	if _, ok := m.Entity(id); !ok {
		return pageView{Status: http.StatusNotFound, Error: fmt.Sprintf("entity %q is not in the model", id)}, nil
	}
	v, err := explorer.Entity(m, id, pathModel)
	return pageView{View: v, Status: http.StatusOK}, err
}

func renderRaw(_ *http.Request, s []source.TargetSummary) (pageView, error) {
	v, err := raw.Render(s)
	return pageView{View: v, Status: http.StatusOK}, err
}

// Handler serves the pages of the workbench, see the package documentation
// (# Pages), the dependency controls at "/controls/dependencies/{name}", and
// every path under inspected.PathPrefix+"/" through inspectedHandler, without
// stripping the prefix. Failed reads of signals or dependency modes are
// logged to logger and shown on the page with status 500.
func Handler(inspectedHandler http.Handler, signals Signals, logger *slog.Logger) http.Handler {
	servePage := func(path string, render renderView) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data := pageData{
				RefreshSeconds: pageRefreshSeconds,
				Tabs:           slices.Clone(tabs),
				Return:         r.URL.RequestURI(),
				Styles:         pageStyles,
			}
			for i := range data.Tabs {
				data.Tabs[i].Active = data.Tabs[i].Path == path
			}
			status := http.StatusOK
			controls, err := dependencyControls(r.Context(), inspectedHandler)
			if err != nil {
				logger.Error("workbench: read dependency modes", "error", err)
				status = http.StatusInternalServerError
				data.ControlsError = fmt.Sprintf("dependency modes unavailable: %v", err)
			}
			data.Controls = controls
			var pv pageView
			summaries, err := signals.Summary(r.Context())
			if err == nil {
				pv, err = render(r, summaries)
			}
			if err != nil {
				logger.Error("workbench: read signals", "error", err)
				status = http.StatusInternalServerError
				data.Error = fmt.Sprintf("signals unavailable: %v", err)
			} else {
				status = max(status, pv.Status)
				data.View, data.Error = pv.View, pv.Error
			}

			var buf bytes.Buffer
			if err := page.Execute(&buf, data); err != nil {
				logger.Error("workbench: render page", "error", err)
				http.Error(w, "render page", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			// A failed write to the client has no recovery.
			_, _ = w.Write(buf.Bytes())
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", servePage(pathDashboard, renderDashboard))
	mux.HandleFunc("GET "+pathModel, servePage(pathModel, renderModel))
	mux.HandleFunc("GET "+pathRaw, servePage(pathRaw, renderRaw))
	mux.HandleFunc("POST /controls/dependencies/{name}", func(w http.ResponseWriter, r *http.Request) {
		setDependencyMode(w, r, inspectedHandler)
	})
	mux.Handle(inspected.PathPrefix+"/", inspectedHandler)
	return mux
}

// exit is how one supervised goroutine stopped.
type exit struct {
	name string // "inspected", "source" or "serve"
	err  error
}

// stopError returns nil when e is a clean stop and an error otherwise.
// stopping reports whether Run had asked the goroutine to stop.
func stopError(e exit, stopping bool) error {
	if stopping && (e.err == nil || errors.Is(e.err, http.ErrServerClosed)) {
		return nil
	}
	if e.err == nil {
		e.err = errors.New("unexpected stop")
	}
	return fmt.Errorf("workbench: %s stopped: %w", e.name, e.err)
}

// Run validates cfg, opens the Source, starts the inspected app, the Source
// and the HTTP server, and blocks until ctx is cancelled or one of them
// fails. Run is the supervisor of all three: an unexpected stop of one cancels
// the others and is returned as an error. A clean shutdown returns nil.
// Restart is not attempted; the process exits and a new start rebuilds the
// inspected state from the seed. logger receives the start and stop records
// of the workbench and the records of the Source; failures before the
// listener is open are only returned.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if logger == nil {
		return errors.New("workbench: logger is required")
	}
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
	// ln.Addr holds the real port when cfg.Addr ends in ":0". The Source reads
	// the inspected service through this listener, like any client.
	// No client timeout: the Source bounds each request with cfg.Source.Timeout.
	// Opening the local file is short; a cancelled ctx means shut down after
	// startup, not fail it, so the open ignores cancellation.
	src, err := source.Open(context.WithoutCancel(ctx), cfg.Source.sourceConfig("http://"+ln.Addr().String()),
		&http.Client{}, logger)
	if err != nil {
		err = fmt.Errorf("workbench: %w", err)
		if closeErr := ln.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("workbench: close listener: %w", closeErr))
		}
		return err
	}
	logger.Info("workbench started", "addr", ln.Addr().String())

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	srv := &http.Server{
		Handler:           Handler(app.Handler(), src, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	const supervised = 3
	exits := make(chan exit, supervised)
	go func() { exits <- exit{"inspected", app.Run(runCtx)} }()
	go func() { exits <- exit{"source", src.Run(runCtx)} }()
	go func() { exits <- exit{"serve", srv.Serve(ln)} }()

	var errs []error
	running := supervised
	select {
	case <-ctx.Done():
	case e := <-exits:
		running--
		// When ctx was cancelled and select picked this ready case first, the
		// goroutine stopped because of the cancellation: a clean shutdown.
		if err := stopError(e, ctx.Err() != nil); err != nil {
			errs = append(errs, err)
		}
	}

	// ctx may already be cancelled, so shutdown needs its own deadline.
	cancel()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("workbench: shutdown: %w", err))
	}

	// Wait for every goroutine: Run owns them.
	for ; running > 0; running-- {
		if err := stopError(<-exits, true); err != nil {
			errs = append(errs, err)
		}
	}
	if err := src.Close(); err != nil {
		errs = append(errs, fmt.Errorf("workbench: %w", err))
	}
	err = errors.Join(errs...)
	if err != nil {
		logger.Error("workbench stopped", "error", err)
	} else {
		logger.Info("workbench stopped")
	}
	return err
}
