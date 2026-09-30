package inspected

import (
	"fmt"
	"net/http"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

func (a *App) handleSimStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	status, err := a.sim.Status(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.simulationResourceOf(status))
}

func (a *App) handleSimClock(w http.ResponseWriter, r *http.Request) {
	var req clockRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	if req.Running == nil {
		writeError(w, fmt.Errorf("%w: running is required", errInvalidRequest))
		return
	}

	ctx, cancel := a.requestContext(r)
	defer cancel()
	status, err := a.sim.SetClockRunning(ctx, *req.Running)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.simulationResourceOf(status))
}

func (a *App) handleSimAdvance(w http.ResponseWriter, r *http.Request) {
	var req advanceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := a.requestContext(r)
	defer cancel()
	status, err := a.sim.Advance(ctx, req.Ticks)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.simulationResourceOf(status))
}

func (a *App) handleSimDependency(w http.ResponseWriter, r *http.Request) {
	var req dependencyModeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	mode, err := fulfillment.ParseDependencyMode(req.Mode)
	if err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := a.requestContext(r)
	defer cancel()
	name := fulfillment.DependencyName(r.PathValue("name"))
	status, err := a.sim.SetDependencyMode(ctx, name, mode)
	if err != nil {
		writeError(w, err)
		return
	}
	for _, d := range status.Dependencies {
		if d.Name == name {
			writeJSON(w, http.StatusOK, dependencyResourceOf(d))
			return
		}
	}
	// SetDependencyMode succeeded, so the dependency must be listed.
	panic(fmt.Sprintf("inspected: dependency %q missing from status", name))
}
