package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

// dependencyControl is the form of one inspected dependency on the page.
type dependencyControl struct {
	Name    string
	Buttons []modeButton // one per fulfillment.DependencyModes, in that order
}

// modeButton is one mode of a dependency. Active marks the current mode.
type modeButton struct {
	Mode   string
	Active bool
}

// inspectedError is a response of inspected with a status of 400 or more.
type inspectedError struct {
	status int
	body   string
}

func (e *inspectedError) Error() string {
	return fmt.Sprintf("inspected answered %d: %s", e.status, e.body)
}

// callInspected serves one request by inspectedHandler in process, like any
// client of inspected, and returns the response body. A status of 400 or more
// is an *inspectedError.
func callInspected(ctx context.Context, inspectedHandler http.Handler, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	rec := httptest.NewRecorder()
	inspectedHandler.ServeHTTP(rec, req)
	if rec.Code >= http.StatusBadRequest {
		return nil, &inspectedError{status: rec.Code, body: rec.Body.String()}
	}
	return rec.Body.Bytes(), nil
}

// dependencyControls reads the dependencies and their current modes from
// GET inspected.PathPrefix+"/sim" and returns one control per dependency.
func dependencyControls(ctx context.Context, inspectedHandler http.Handler) ([]dependencyControl, error) {
	body, err := callInspected(ctx, inspectedHandler, http.MethodGet, inspected.PathPrefix+"/sim", nil)
	if err != nil {
		return nil, err
	}
	var status struct {
		Dependencies []struct {
			Name string `json:"name"`
			Mode string `json:"mode"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("decode simulation status: %w", err)
	}
	controls := make([]dependencyControl, 0, len(status.Dependencies))
	for _, d := range status.Dependencies {
		c := dependencyControl{Name: d.Name}
		for _, mode := range fulfillment.DependencyModes() {
			c.Buttons = append(c.Buttons, modeButton{Mode: string(mode), Active: string(mode) == d.Mode})
		}
		controls = append(controls, c)
	}
	return controls, nil
}

// setDependencyMode sets the mode of the dependency {name} to the form value
// "mode" through PUT inspected.PathPrefix+"/sim/dependencies/{name}". Success
// redirects to the page with 303. A rejected request answers with the status
// of inspected and its response body as text.
func setDependencyMode(w http.ResponseWriter, r *http.Request, inspectedHandler http.Handler) {
	name, mode := r.PathValue("name"), r.FormValue("mode")
	body, err := json.Marshal(struct {
		Mode string `json:"mode"`
	}{mode})
	if err != nil {
		// A struct of one string always encodes.
		panic(fmt.Sprintf("workbench: encode mode: %v", err))
	}
	_, err = callInspected(r.Context(), inspectedHandler, http.MethodPut,
		inspected.PathPrefix+"/sim/dependencies/"+url.PathEscape(name), body)
	if err != nil {
		status := http.StatusInternalServerError
		if ie, ok := errors.AsType[*inspectedError](err); ok {
			status = ie.status
		}
		http.Error(w, fmt.Sprintf("set %s to %s: %v", name, mode, err), status)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
