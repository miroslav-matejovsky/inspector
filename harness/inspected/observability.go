package inspected

import (
	"net/http"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/health"
)

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, indexResource{
		Service:     "inspected",
		Description: "Simulated order fulfillment service",
		Links: map[string]string{
			"self":         PathPrefix + "/",
			"health_live":  PathPrefix + pathHealthLive,
			"health_ready": PathPrefix + pathHealthReady,
			"metrics":      PathPrefix + pathMetrics,
			"simulation":   PathPrefix + pathSim,
			"dependencies": PathPrefix + pathDependencies,
			"products":     PathPrefix + pathProducts,
			"orders":       PathPrefix + pathOrders,
		},
	})
}

// handleLive reports whether the simulation actor answers.
func (a *App) handleLive(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	if _, err := a.sim.Status(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, liveResource{Status: "down", Reason: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, liveResource{Status: "up"})
}

// handleReady reports the readiness evaluated from the current state. A down
// report is unavailable; up and degraded are available.
func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResource{
			Status: string(health.StatusDown),
			Checks: []checkResource{{Name: "simulation", Status: string(health.StatusDown), Reason: err.Error()}},
		})
		return
	}

	report := health.Evaluate(snap)
	status := http.StatusOK
	if report.Status == health.StatusDown {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, healthResourceOf(report))
}
