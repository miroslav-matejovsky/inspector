package representation

import (
	"net/url"
	"time"

	"github.com/miroslav-matejovsky/inspector/observation"
)

// JSON resources of the views. Lists are always non-nil so they encode as
// [] and never as null.

type overviewResource struct {
	ObservedAt   time.Time      `json:"observed_at"`
	EntitiesHref string         `json:"entities_href"`
	Kinds        []kindResource `json:"kinds"`
}

type kindResource struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Href  string `json:"href"`
}

type entityListResource struct {
	ObservedAt time.Time       `json:"observed_at"`
	Entities   []entitySummary `json:"entities"`
}

type entitySummary struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	State string `json:"state,omitempty"`
	Href  string `json:"href"`
}

type entityDetailResource struct {
	ObservedAt time.Time         `json:"observed_at"`
	Entity     entityResource    `json:"entity"`
	Related    []relatedResource `json:"related"`
}

type entityResource struct {
	Kind       string              `json:"kind"`
	ID         string              `json:"id"`
	State      string              `json:"state,omitempty"`
	Href       string              `json:"href"`
	Attributes []attributeResource `json:"attributes"`
}

type attributeResource struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type relatedResource struct {
	Relation  string      `json:"relation"`
	Direction string      `json:"direction"`
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

func (h *handler) entitiesHref() string {
	return h.prefix + "/entities"
}

func (h *handler) kindHref(kind string) string {
	return h.entitiesHref() + "?kind=" + url.QueryEscape(kind)
}

func (h *handler) entityHref(ref observation.Ref) string {
	return h.entitiesHref() + "/" + url.PathEscape(ref.Kind) + "/" + url.PathEscape(ref.ID)
}

func (h *handler) refResourceOf(ref observation.Ref) refResource {
	return refResource{Kind: ref.Kind, ID: ref.ID, Href: h.entityHref(ref)}
}
