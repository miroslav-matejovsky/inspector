package inspected

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/metrics"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/simulation"
)

// PathPrefix isolates every inspected endpoint. The workbench mounts Handler
// at PathPrefix + "/"; the routes of Handler include the prefix.
const PathPrefix = "/inspected"

// App is the inspected service: the simulation actor plus its HTTP surface.
type App struct {
	cfg      Config
	registry *prometheus.Registry
	metrics  *metrics.Metrics
	sim      *simulation.Simulation
}

// New validates cfg and wires the service. It starts no goroutine; call Run.
func New(cfg Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	registry := prometheus.NewRegistry()
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	} {
		if err := registry.Register(c); err != nil {
			return nil, fmt.Errorf("inspected: register runtime collector: %w", err)
		}
	}

	m, err := metrics.New(registry)
	if err != nil {
		return nil, fmt.Errorf("inspected: %w", err)
	}

	sim, err := simulation.New(
		simulation.Config{Seed: cfg.Seed, OrdersPerTick: cfg.OrdersPerTick},
		fulfillment.StandardCatalog(),
		m,
	)
	if err != nil {
		return nil, fmt.Errorf("inspected: %w", err)
	}

	return &App{cfg: cfg, registry: registry, metrics: m, sim: sim}, nil
}

// Run drives the simulation clock and blocks until ctx is cancelled. It
// returns nil on cancellation and an error when the simulation fails. The
// caller is the supervisor: state is in memory, so a restart is a new App.
func (a *App) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.TickInterval)
	defer ticker.Stop()
	return a.sim.Run(ctx, ticker.C)
}

// Handler returns the HTTP handler of every inspected endpoint. Each route is
// instrumented under its route name.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range a.routes() {
		mux.Handle(r.pattern, a.metrics.InstrumentHandler(r.name, r.handler))
	}
	return mux
}

// requestContext bounds the wait for a simulation reply.
func (a *App) requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), a.cfg.RequestTimeout)
}
