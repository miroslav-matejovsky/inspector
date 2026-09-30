package workbench

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/source"
)

// Kinds of the entities of the inspected system.
const (
	kindService    = "service"    // the index: /inspected/
	kindReadiness  = "readiness"  // the readiness report: /inspected/health/ready
	kindCheck      = "check"      // one check of the readiness report
	kindDependency = "dependency" // a downstream system of inspected
	kindProduct    = "product"
	kindOrder      = "order"
)

// Link types, read from the entity that holds the link.
const (
	linkReadiness    = "readiness"     // service -> readiness
	linkCheck        = "check"         // readiness -> check
	linkDeterminedBy = "determined by" // check -> dependency or product ("causes")
	linkProduct      = "product"       // order -> product ("links.product")
	linkWaitsOn      = "waits on"      // order -> dependency ("links.waiting_on")
	linkDecidedBy    = "decided by"    // order -> dependency or product (history "cause")
)

// Health of the status words of inspected. A word that is missing maps to
// the zero Health, which model.New reads as unknown.
var (
	statusHealth = map[string]model.Health{"up": model.HealthOK, "degraded": model.HealthDegraded, "down": model.HealthDown}
	modeHealth   = map[string]model.Health{"healthy": model.HealthOK, "slow": model.HealthDegraded, "outage": model.HealthDown}
	orderHealth  = map[string]model.Health{
		"pending": model.HealthOK, "paid": model.HealthOK, "shipped": model.HealthOK, "failed": model.HealthDown,
	}
)

// inspectedInterpreters maps the URL path of a target to the interpreter of
// its body. The paths are observation endpoints of package inspected.
var inspectedInterpreters = map[string]model.Interpreter{
	inspected.PathPrefix + "/":                 interpretIndex,
	inspected.PathPrefix + "/health/ready":     interpretReadiness,
	inspected.PathPrefix + "/api/dependencies": interpretDependencies,
	inspected.PathPrefix + "/api/products":     interpretProducts,
	inspected.PathPrefix + "/api/orders":       interpretOrders,
}

// inspectedInterpreter returns the interpreter of t by the path of its URL,
// or nil when the path is not in inspectedInterpreters. The query is
// ignored.
func inspectedInterpreter(t source.Target) model.Interpreter {
	u, err := url.Parse(t.URL)
	if err != nil {
		// source.Config.Validate accepts only URLs that parse.
		return nil
	}
	return inspectedInterpreters[u.Path]
}

// decodeBody decodes the JSON body of sig into v when the status of sig is
// one of ok. Unknown fields are ignored: inspected may state more than the
// model reads.
func decodeBody(sig source.Signal, v any, ok ...int) error {
	if !slices.Contains(ok, sig.StatusCode) {
		return fmt.Errorf("status %d", sig.StatusCode)
	}
	if err := json.Unmarshal(sig.Body, v); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	return nil
}

// appendLink appends the link {typ, to} to links, unless to is empty or
// links already holds it.
func appendLink(links []model.Link, typ, to string) []model.Link {
	l := model.Link{Type: typ, To: to}
	if to == "" || slices.Contains(links, l) {
		return links
	}
	return append(links, l)
}

// interpretIndex reads the service from the index of inspected.
func interpretIndex(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Service     string            `json:"service"`
		Description string            `json:"description"`
		Links       map[string]string `json:"links"`
	}
	if err := decodeBody(sig, &body, http.StatusOK); err != nil {
		return nil, err
	}
	e := model.Entity{
		ID:         body.Links["self"],
		Kind:       kindService,
		Name:       body.Service,
		Properties: []model.Property{{Name: "description", Value: body.Description}},
	}
	e.Links = appendLink(e.Links, linkReadiness, body.Links["health_ready"])
	return []model.Entity{e}, nil
}

// interpretReadiness reads the readiness report and its checks. The report
// states no self link, so its ID is the path of the signal URL, which is the
// link of the index. Status 503 is a report that is not ready.
func interpretReadiness(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string   `json:"name"`
			Status string   `json:"status"`
			Reason string   `json:"reason"`
			Causes []string `json:"causes"`
		} `json:"checks"`
	}
	if err := decodeBody(sig, &body, http.StatusOK, http.StatusServiceUnavailable); err != nil {
		return nil, err
	}
	u, err := url.Parse(sig.URL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	id := u.Path
	report := model.Entity{
		ID:    id,
		Kind:  kindReadiness,
		Name:  "readiness",
		State: model.State{Value: body.Status, Health: statusHealth[body.Status]},
	}
	checks := make([]model.Entity, 0, len(body.Checks))
	for _, c := range body.Checks {
		check := model.Entity{
			ID:    id + "#" + c.Name,
			Kind:  kindCheck,
			Name:  c.Name,
			State: model.State{Value: c.Status, Health: statusHealth[c.Status], Reason: c.Reason},
		}
		for _, cause := range c.Causes {
			check.Links = appendLink(check.Links, linkDeterminedBy, cause)
		}
		report.Links = appendLink(report.Links, linkCheck, check.ID)
		checks = append(checks, check)
	}
	return append([]model.Entity{report}, checks...), nil
}

// interpretDependencies reads the downstream systems of inspected.
func interpretDependencies(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Dependencies []struct {
			Name  string `json:"name"`
			Mode  string `json:"mode"`
			Links struct {
				Self string `json:"self"`
			} `json:"links"`
		} `json:"dependencies"`
	}
	if err := decodeBody(sig, &body, http.StatusOK); err != nil {
		return nil, err
	}
	entities := make([]model.Entity, 0, len(body.Dependencies))
	for _, d := range body.Dependencies {
		entities = append(entities, model.Entity{
			ID:    d.Links.Self,
			Kind:  kindDependency,
			Name:  d.Name,
			State: model.State{Value: d.Mode, Health: modeHealth[d.Mode]},
		})
	}
	return entities, nil
}

// interpretProducts reads the catalog with its stock numbers. Products
// state no health.
func interpretProducts(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Products []struct {
			SKU          string `json:"sku"`
			Name         string `json:"name"`
			PriceCents   int64  `json:"price_cents"`
			Stock        int    `json:"stock"`
			Capacity     int    `json:"capacity"`
			ReorderPoint int    `json:"reorder_point"`
			Links        struct {
				Self string `json:"self"`
			} `json:"links"`
		} `json:"products"`
	}
	if err := decodeBody(sig, &body, http.StatusOK); err != nil {
		return nil, err
	}
	entities := make([]model.Entity, 0, len(body.Products))
	for _, p := range body.Products {
		entities = append(entities, model.Entity{
			ID:   p.Links.Self,
			Kind: kindProduct,
			Name: p.SKU,
			Properties: []model.Property{
				{Name: "name", Value: p.Name},
				{Name: "price_cents", Value: strconv.FormatInt(p.PriceCents, 10)},
				{Name: "stock", Value: strconv.Itoa(p.Stock)},
				{Name: "capacity", Value: strconv.Itoa(p.Capacity)},
				{Name: "reorder_point", Value: strconv.Itoa(p.ReorderPoint)},
			},
		})
	}
	return entities, nil
}

// interpretOrders reads the orders, newest first, with their history as
// properties and the resources they relate to as links.
func interpretOrders(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Orders []struct {
			ID            string `json:"id"`
			SKU           string `json:"sku"`
			Quantity      int    `json:"quantity"`
			TotalCents    int64  `json:"total_cents"`
			Channel       string `json:"channel"`
			Status        string `json:"status"`
			PlacedAtTick  uint64 `json:"placed_at_tick"`
			UpdatedAtTick uint64 `json:"updated_at_tick"`
			FailureReason string `json:"failure_reason"`
			History       []struct {
				From   string `json:"from"`
				To     string `json:"to"`
				AtTick uint64 `json:"at_tick"`
				Reason string `json:"reason"`
				Cause  string `json:"cause"`
			} `json:"history"`
			Links struct {
				Self      string `json:"self"`
				Product   string `json:"product"`
				WaitingOn string `json:"waiting_on"`
			} `json:"links"`
		} `json:"orders"`
	}
	if err := decodeBody(sig, &body, http.StatusOK); err != nil {
		return nil, err
	}
	entities := make([]model.Entity, 0, len(body.Orders))
	// The list is in placement order. Newest first, so that a view that shows
	// the first entities of a kind shows the recent orders.
	for _, o := range slices.Backward(body.Orders) {
		e := model.Entity{
			ID:    o.Links.Self,
			Kind:  kindOrder,
			Name:  o.ID,
			State: model.State{Value: o.Status, Health: orderHealth[o.Status], Reason: o.FailureReason},
			Properties: []model.Property{
				{Name: "sku", Value: o.SKU},
				{Name: "quantity", Value: strconv.Itoa(o.Quantity)},
				{Name: "total_cents", Value: strconv.FormatInt(o.TotalCents, 10)},
				{Name: "channel", Value: o.Channel},
				{Name: "placed_at_tick", Value: strconv.FormatUint(o.PlacedAtTick, 10)},
				{Name: "updated_at_tick", Value: strconv.FormatUint(o.UpdatedAtTick, 10)},
			},
		}
		for _, h := range o.History {
			step := h.To + " (" + h.Reason + ")"
			if h.From != "" {
				step = h.From + " -> " + step
			}
			e.Properties = append(e.Properties, model.Property{Name: "tick " + strconv.FormatUint(h.AtTick, 10), Value: step})
		}
		e.Links = appendLink(e.Links, linkProduct, o.Links.Product)
		e.Links = appendLink(e.Links, linkWaitsOn, o.Links.WaitingOn)
		for _, h := range o.History {
			e.Links = appendLink(e.Links, linkDecidedBy, h.Cause)
		}
		entities = append(entities, e)
	}
	return entities, nil
}
