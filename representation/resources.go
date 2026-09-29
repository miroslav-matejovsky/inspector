package representation

import (
	"net/url"
	"time"

	"github.com/miroslav-matejovsky/inspector/explanation"
	"github.com/miroslav-matejovsky/inspector/observation"
)

// JSON resources of the views. Lists are always non-nil so they encode as
// [] and never as null.

type overviewResource struct {
	ObservedAt   time.Time      `json:"observed_at"`
	Gaps         []gapResource  `json:"gaps"`
	EntitiesHref string         `json:"entities_href"`
	Kinds        []kindResource `json:"kinds"`
}

type gapResource struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type kindResource struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Href  string `json:"href"`
}

type entityListResource struct {
	ObservedAt time.Time       `json:"observed_at"`
	Gaps       []gapResource   `json:"gaps"`
	Entities   []entitySummary `json:"entities"`
}

type entitySummary struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	State string `json:"state,omitempty"`
	Href  string `json:"href"`
}

type entityDetailResource struct {
	ObservedAt      time.Time         `json:"observed_at"`
	Gaps            []gapResource     `json:"gaps"`
	Entity          entityResource    `json:"entity"`
	Related         []relatedResource `json:"related"`
	ExplanationHref string            `json:"explanation_href"`
}

type explanationViewResource struct {
	ObservedAt  time.Time           `json:"observed_at"`
	Gaps        []gapResource       `json:"gaps"`
	Explanation explanationResource `json:"explanation"`
	RootCauses  []rootCauseResource `json:"root_causes"` // never null
}

type explanationResource struct {
	Kind    string               `json:"kind"`
	ID      string               `json:"id"`
	State   string               `json:"state,omitempty"`
	Reason  string               `json:"reason,omitempty"`
	Href    string               `json:"href"`
	History []transitionResource `json:"history"` // never null
	Causes  []causeResource      `json:"causes"`  // never null
}

type causeResource struct {
	Relation            string `json:"relation"`
	Repeated            bool   `json:"repeated,omitempty"`
	explanationResource        // embedded: its fields appear at the same level
}

type rootCauseResource struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	State string `json:"state,omitempty"`
	Href  string `json:"href"`
}

// explanationResourceOf converts an explanation tree, keeping every list
// non-nil.
func (h *handler) explanationResourceOf(e explanation.Explanation) explanationResource {
	causes := make([]causeResource, 0, len(e.Causes))
	for _, c := range e.Causes {
		causes = append(causes, causeResource{
			Relation: c.Relation, Repeated: c.Repeated, explanationResource: h.explanationResourceOf(c.Explanation),
		})
	}
	return explanationResource{
		Kind: e.Ref.Kind, ID: e.Ref.ID, State: e.State, Reason: e.Reason, Href: h.entityHref(e.Ref),
		History: historyOf(e.History), Causes: causes,
	}
}

type entityResource struct {
	Kind       string               `json:"kind"`
	ID         string               `json:"id"`
	State      string               `json:"state,omitempty"`
	Reason     string               `json:"reason,omitempty"`
	Href       string               `json:"href"`
	Attributes []attributeResource  `json:"attributes"`
	History    []transitionResource `json:"history"`
}

type transitionResource struct {
	From   string `json:"from,omitempty"`
	To     string `json:"to"`
	At     string `json:"at,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// historyOf returns the transitions in order, never nil.
func historyOf(history []observation.Transition) []transitionResource {
	out := make([]transitionResource, 0, len(history))
	for _, t := range history {
		out = append(out, transitionResource{From: t.From, To: t.To, At: t.At, Reason: t.Reason})
	}
	return out
}

type attributeResource struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type relatedResource struct {
	Relation  string      `json:"relation"`
	Direction string      `json:"direction"`
	Cause     bool        `json:"cause,omitempty"`
	Target    refResource `json:"target"`
}

type refResource struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Href string `json:"href"`
}

type errorResource struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// gapsOf returns the gaps of s in order, never nil.
func gapsOf(s observation.Snapshot) []gapResource {
	gaps := make([]gapResource, 0, len(s.Gaps()))
	for _, g := range s.Gaps() {
		gaps = append(gaps, gapResource{Source: g.Source, Error: g.Error})
	}
	return gaps
}

func (h *handler) entitiesHref() string {
	return h.prefix + "/entities"
}

func (h *handler) kindHref(kind string) string {
	return h.entitiesHref() + "?kind=" + url.QueryEscape(kind)
}

func (h *handler) entityHref(ref observation.Ref) string {
	return h.entitiesHref() + "/" + url.PathEscape(ref.Kind) + "/" + url.PathEscape(ref.ID)
}

func (h *handler) explanationHref(ref observation.Ref) string {
	return h.entityHref(ref) + "/explanation"
}

func (h *handler) refResourceOf(ref observation.Ref) refResource {
	return refResource{Kind: ref.Kind, ID: ref.ID, Href: h.entityHref(ref)}
}
