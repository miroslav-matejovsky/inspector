package representation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/miroslav-matejovsky/inspector/explanation"
	"github.com/miroslav-matejovsky/inspector/navigation"
	"github.com/miroslav-matejovsky/inspector/observation"
)

// Observer provides the current snapshot of an inspected system. The handler
// calls it once per request, with the request context.
type Observer interface {
	Observe(ctx context.Context) (observation.Snapshot, error)
}

var prefixPattern = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)+$`)

type handler struct {
	prefix   string
	observer Observer
}

// NewHandler returns the JSON views of the snapshots of observer, served under
// prefix, for example "/inspector". prefix is one or more path segments of
// letters, digits, ".", "_", "~" or "-", each starting with "/", without a
// trailing slash. The host mounts the handler at prefix+"/" and does not strip
// the prefix. observer must not be nil.
func NewHandler(prefix string, observer Observer) (http.Handler, error) {
	if !prefixPattern.MatchString(prefix) {
		return nil, fmt.Errorf("representation: prefix %q must match %s", prefix, prefixPattern)
	}
	if observer == nil {
		return nil, errors.New("representation: observer is required")
	}
	h := &handler{prefix: prefix, observer: observer}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix+"/{$}", h.handleOverview)
	mux.HandleFunc("GET "+prefix+"/entities", h.handleEntities)
	mux.HandleFunc("GET "+prefix+"/entities/{kind}/{id}", h.handleEntity)
	mux.HandleFunc("GET "+prefix+"/entities/{kind}/{id}/explanation", h.handleExplanation)
	return mux, nil
}

// observe returns the current snapshot or writes a 503 and returns false.
func (h *handler) observe(w http.ResponseWriter, r *http.Request) (observation.Snapshot, bool) {
	snap, err := h.observer.Observe(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "source_unavailable", err.Error())
		return observation.Snapshot{}, false
	}
	return snap, true
}

// handleOverview lists the kinds in order of first appearance with their
// entity counts.
func (h *handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	snap, ok := h.observe(w, r)
	if !ok {
		return
	}
	kinds := []kindResource{}
	position := map[string]int{}
	for _, e := range snap.Entities() {
		i, seen := position[e.Ref.Kind]
		if !seen {
			i = len(kinds)
			position[e.Ref.Kind] = i
			kinds = append(kinds, kindResource{Kind: e.Ref.Kind, Href: h.kindHref(e.Ref.Kind)})
		}
		kinds[i].Count++
	}
	writeJSON(w, http.StatusOK, overviewResource{
		ObservedAt:   snap.ObservedAt(),
		Gaps:         gapsOf(snap),
		EntitiesHref: h.entitiesHref(),
		Kinds:        kinds,
	})
}

// handleEntities lists entities in snapshot order, filtered by the optional
// query parameter kind.
func (h *handler) handleEntities(w http.ResponseWriter, r *http.Request) {
	snap, ok := h.observe(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	entities := []entitySummary{}
	for _, e := range snap.Entities() {
		if kind != "" && e.Ref.Kind != kind {
			continue
		}
		entities = append(entities, entitySummary{
			Kind: e.Ref.Kind, ID: e.Ref.ID, State: e.State, Href: h.entityHref(e.Ref),
		})
	}
	writeJSON(w, http.StatusOK, entityListResource{ObservedAt: snap.ObservedAt(), Gaps: gapsOf(snap), Entities: entities})
}

// handleEntity shows one entity with its attributes and related entities.
func (h *handler) handleEntity(w http.ResponseWriter, r *http.Request) {
	snap, ok := h.observe(w, r)
	if !ok {
		return
	}
	ref := observation.Ref{Kind: r.PathValue("kind"), ID: r.PathValue("id")}
	links, err := navigation.Links(snap, ref)
	switch {
	case errors.Is(err, navigation.ErrUnknownEntity):
		writeError(w, http.StatusNotFound, "entity_not_found", fmt.Sprintf("entity %s not found", ref))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	entity, _ := snap.Entity(ref) // exists: Links succeeded

	attributes := make([]attributeResource, 0, len(entity.Attributes))
	for _, a := range entity.Attributes {
		attributes = append(attributes, attributeResource{Name: a.Name, Value: a.Value})
	}
	related := make([]relatedResource, 0, len(links))
	for _, l := range links {
		related = append(related, relatedResource{
			Relation: l.Relation, Direction: string(l.Direction), Cause: l.Cause, Target: h.refResourceOf(l.Target),
		})
	}
	writeJSON(w, http.StatusOK, entityDetailResource{
		ObservedAt: snap.ObservedAt(),
		Gaps:       gapsOf(snap),
		Entity: entityResource{
			Kind: ref.Kind, ID: ref.ID, State: entity.State, Reason: entity.Reason, Href: h.entityHref(ref),
			Attributes: attributes, History: historyOf(entity.History),
		},
		Related:         related,
		ExplanationHref: h.explanationHref(ref),
	})
}

// handleExplanation shows why one entity is in its state: the cause tree and
// the root causes.
func (h *handler) handleExplanation(w http.ResponseWriter, r *http.Request) {
	snap, ok := h.observe(w, r)
	if !ok {
		return
	}
	ref := observation.Ref{Kind: r.PathValue("kind"), ID: r.PathValue("id")}
	e, err := explanation.Explain(snap, ref)
	switch {
	case errors.Is(err, explanation.ErrUnknownEntity):
		writeError(w, http.StatusNotFound, "entity_not_found", fmt.Sprintf("entity %s not found", ref))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	roots := []rootCauseResource{}
	for _, root := range e.Roots() {
		entity, _ := snap.Entity(root) // roots are entities of the snapshot
		roots = append(roots, rootCauseResource{Kind: root.Kind, ID: root.ID, State: entity.State, Href: h.entityHref(root)})
	}
	writeJSON(w, http.StatusOK, explanationViewResource{
		ObservedAt:  snap.ObservedAt(),
		Gaps:        gapsOf(snap),
		Explanation: h.explanationResourceOf(e),
		RootCauses:  roots,
	})
}

// writeJSON writes v followed by a newline. A value that cannot be encoded,
// for example a time outside the years 0 to 9999, is a 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeBody(w, status, body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	// errorResource holds only strings, so encoding cannot fail.
	body, _ := json.Marshal(errorResource{Error: errorBody{Code: code, Message: message}})
	writeBody(w, status, body)
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// A failed write means the client is gone; there is nobody to report to.
	_, _ = w.Write(append(body, '\n'))
}
