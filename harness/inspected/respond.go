package inspected

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/simulation"
)

// maxBodyBytes bounds a request body.
const maxBodyBytes = 1 << 20

// errInvalidRequest marks a malformed request body or missing field.
var errInvalidRequest = errors.New("invalid request")

// writeJSON writes v as the response body followed by a newline.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		// v is always one of the plain resource types of this package, so
		// this is a programming error.
		panic(fmt.Sprintf("inspected: encode %T: %v", v, err))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// A failed write means the client is gone; there is nobody to report to.
	_, _ = w.Write(append(body, '\n'))
}

// writeError writes err with the status and code chosen by errorResponse.
func writeError(w http.ResponseWriter, err error) {
	status, code := errorResponse(err)
	writeErrorCode(w, status, code, err.Error())
}

func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResource{Error: errorBody{Code: code, Message: message}})
}

// errorResponse maps an error to an HTTP status and a stable error code.
func errorResponse(err error) (status int, code string) {
	switch {
	case errors.Is(err, errInvalidRequest):
		return http.StatusBadRequest, "invalid_request"
	case errors.Is(err, simulation.ErrStopped),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		return http.StatusServiceUnavailable, "simulation_unavailable"
	case errors.Is(err, simulation.ErrInvalidAdvance):
		return http.StatusUnprocessableEntity, "invalid_advance"
	case errors.Is(err, fulfillment.ErrUnknownDependency):
		return http.StatusNotFound, "unknown_dependency"
	case errors.Is(err, fulfillment.ErrInvalidDependencyMode):
		return http.StatusUnprocessableEntity, "invalid_dependency_mode"
	case errors.Is(err, fulfillment.ErrUnknownProduct):
		return http.StatusUnprocessableEntity, "unknown_product"
	case errors.Is(err, fulfillment.ErrInvalidQuantity):
		return http.StatusUnprocessableEntity, "invalid_quantity"
	case errors.Is(err, fulfillment.ErrInvalidOrderStatus):
		return http.StatusBadRequest, "invalid_status"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// decodeJSON reads exactly one JSON value from the request body into dst.
// Unknown fields, trailing data and an empty body are errors wrapping
// errInvalidRequest.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %s", errInvalidRequest, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: body must contain a single JSON value", errInvalidRequest)
	}
	return nil
}
