package adapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/miroslav-matejovsky/inspector/connectivity"
	"github.com/miroslav-matejovsky/inspector/observation"
)

// Vocabulary of the inspected service in the observation model, and the
// paths the adapter reads, relative to the inspected path prefix.
const (
	kindService     = "service"
	kindHealthCheck = "health_check"
	kindProduct     = "product"
	kindOrder       = "order"

	relationHasCheck   = "has_check"
	relationForProduct = "for_product"

	serviceID = "inspected" // the inspected service is the only service

	pathReadiness = "/health/ready"
	pathProducts  = "/api/products"
	pathOrders    = "/api/orders"
)

// Adapter observes the inspected service through its read-only HTTP API.
type Adapter struct {
	reader *connectivity.HTTPReader // base URL is the inspected path prefix
	now    func() time.Time
}

// New returns an adapter that reads with reader and stamps each snapshot with
// now. Both are required.
func New(reader *connectivity.HTTPReader, now func() time.Time) (*Adapter, error) {
	if reader == nil {
		return nil, errors.New("adapter: reader is required")
	}
	if now == nil {
		return nil, errors.New("adapter: clock is required")
	}
	return &Adapter{reader: reader, now: now}, nil
}

// Observe reads readiness, products and orders, in this order, and returns
// them as one snapshot stamped with now() after the last read. The three
// reads are not atomic: they can reflect different simulation ticks.
func (a *Adapter) Observe(ctx context.Context) (observation.Snapshot, error) {
	var health healthDoc
	// Readiness down answers 503 with a full report: that is an observation.
	if err := a.read(ctx, pathReadiness, &health, http.StatusOK, http.StatusServiceUnavailable); err != nil {
		return observation.Snapshot{}, err
	}
	var products productListDoc
	if err := a.read(ctx, pathProducts, &products, http.StatusOK); err != nil {
		return observation.Snapshot{}, err
	}
	var orders orderListDoc
	if err := a.read(ctx, pathOrders, &orders, http.StatusOK); err != nil {
		return observation.Snapshot{}, err
	}

	var entities []observation.Entity
	var relations []observation.Relation

	service, err := mapHealth(health)
	if err != nil {
		return observation.Snapshot{}, err
	}
	entities = append(entities, service...)
	for _, c := range health.Checks {
		relations = append(relations, observation.Relation{
			From: observation.Ref{Kind: kindService, ID: serviceID},
			Kind: relationHasCheck,
			To:   observation.Ref{Kind: kindHealthCheck, ID: c.Name},
		})
	}

	for i, p := range products.Products {
		if p.SKU == "" {
			return observation.Snapshot{}, missing(pathProducts, "product", i, "sku")
		}
		entities = append(entities, observation.Entity{
			Ref: observation.Ref{Kind: kindProduct, ID: p.SKU},
			Attributes: []observation.Attribute{
				{Name: "name", Value: p.Name},
				{Name: "price_cents", Value: strconv.FormatInt(p.PriceCents, 10)},
				{Name: "stock", Value: strconv.Itoa(p.Stock)},
				{Name: "capacity", Value: strconv.Itoa(p.Capacity)},
				{Name: "reorder_point", Value: strconv.Itoa(p.ReorderPoint)},
			},
		})
	}

	for i, o := range orders.Orders {
		switch {
		case o.ID == "":
			return observation.Snapshot{}, missing(pathOrders, "order", i, "id")
		case o.SKU == "":
			return observation.Snapshot{}, missing(pathOrders, "order", i, "sku")
		case o.Status == "":
			return observation.Snapshot{}, missing(pathOrders, "order", i, "status")
		}
		attrs := []observation.Attribute{
			{Name: "quantity", Value: strconv.Itoa(o.Quantity)},
			{Name: "total_cents", Value: strconv.FormatInt(o.TotalCents, 10)},
			{Name: "channel", Value: o.Channel},
			{Name: "placed_at_tick", Value: strconv.FormatUint(o.PlacedAtTick, 10)},
			{Name: "updated_at_tick", Value: strconv.FormatUint(o.UpdatedAtTick, 10)},
		}
		if o.FailureReason != "" {
			attrs = append(attrs, observation.Attribute{Name: "failure_reason", Value: o.FailureReason})
		}
		ref := observation.Ref{Kind: kindOrder, ID: o.ID}
		entities = append(entities, observation.Entity{Ref: ref, State: o.Status, Attributes: attrs})
		relations = append(relations, observation.Relation{
			From: ref,
			Kind: relationForProduct,
			To:   observation.Ref{Kind: kindProduct, ID: o.SKU},
		})
	}

	snap, err := observation.NewSnapshot(a.now(), entities, relations)
	if err != nil {
		return observation.Snapshot{}, fmt.Errorf("adapter: %w", err)
	}
	return snap, nil
}

// read gets path, requires one of the accepted statuses and decodes the body
// into dst.
func (a *Adapter) read(ctx context.Context, path string, dst any, accepted ...int) error {
	doc, err := a.reader.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("adapter: read %s: %w", path, err)
	}
	if !slices.Contains(accepted, doc.Status) {
		return fmt.Errorf("adapter: read %s: unexpected status %d", path, doc.Status)
	}
	if err := doc.Decode(dst); err != nil {
		return fmt.Errorf("adapter: read %s: %w", path, err)
	}
	return nil
}

// mapHealth maps the readiness report to the service entity followed by one
// entity per check.
func mapHealth(h healthDoc) ([]observation.Entity, error) {
	if h.Status == "" {
		return nil, fmt.Errorf("adapter: read %s: readiness without status", pathReadiness)
	}
	entities := []observation.Entity{{
		Ref:   observation.Ref{Kind: kindService, ID: serviceID},
		State: h.Status,
	}}
	for i, c := range h.Checks {
		switch {
		case c.Name == "":
			return nil, missing(pathReadiness, "check", i, "name")
		case c.Status == "":
			return nil, missing(pathReadiness, "check", i, "status")
		}
		var attrs []observation.Attribute
		if c.Reason != "" {
			attrs = append(attrs, observation.Attribute{Name: "reason", Value: c.Reason})
		}
		entities = append(entities, observation.Entity{
			Ref:        observation.Ref{Kind: kindHealthCheck, ID: c.Name},
			State:      c.Status,
			Attributes: attrs,
		})
	}
	return entities, nil
}

func missing(path, item string, index int, field string) error {
	return fmt.Errorf("adapter: read %s: %s %d without %s", path, item, index, field)
}
