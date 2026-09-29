package adapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/inspector/connectivity"
	"github.com/miroslav-matejovsky/inspector/observation"
)

// Vocabulary of the inspected service in the observation model, and the
// paths the adapter reads, relative to the inspected path prefix.
const (
	kindService     = "service"
	kindHealthCheck = "health_check"
	kindDependency  = "dependency"
	kindProduct     = "product"
	kindOrder       = "order"

	relationHasCheck   = "has_check"
	relationForProduct = "for_product"
	relationCausedBy   = "caused_by"
	relationWaitsOn    = "waits_on"

	serviceID = "inspected" // the inspected service is the only service

	pathReadiness    = "/health/ready"
	pathDependencies = "/api/dependencies"
	pathProducts     = "/api/products"
	pathOrders       = "/api/orders"
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

// build collects the parts of one snapshot.
type build struct {
	entities  []observation.Entity
	relations []observation.Relation
	gaps      []observation.Gap
	observed  map[observation.Ref]bool
	readOK    map[string]bool // kind -> whether the read that yields this kind succeeded
}

func (b *build) gap(source, format string, args ...any) {
	b.gaps = append(b.gaps, observation.Gap{Source: source, Error: fmt.Sprintf(format, args...)})
}

func (b *build) add(e observation.Entity) {
	b.entities = append(b.entities, e)
	b.observed[e.Ref] = true
}

// relate adds the relation from -kind-> to when to was observed. When to is
// missing although the read of its kind succeeded, it records a gap under
// source instead. When that read failed, its gap already explains the missing
// relation and nothing is added.
func (b *build) relate(source string, from observation.Ref, kind string, to observation.Ref, cause bool) {
	switch {
	case b.observed[to]:
		b.relations = append(b.relations, observation.Relation{From: from, Kind: kind, To: to, Cause: cause})
	case b.readOK[to.Kind]:
		b.gap(source, "%s: %s target %s not observed", from, kind, to)
	}
}

// relateLink is relate for a target given as a link of the inspected API. A
// link that names no entity is recorded as a gap.
func (b *build) relateLink(source string, from observation.Ref, kind, link string, cause bool) {
	to, ok := refOf(link)
	if !ok {
		b.gap(source, "%s: unknown link %q", from, link)
		return
	}
	b.relate(source, from, kind, to, cause)
}

// refOf maps a link of the inspected API to the entity it names. A link that
// ends in /api/dependencies/<name> names dependency/<name>; a link that ends
// in /api/products/<sku> names product/<sku>. The last segment is path
// unescaped and must be non-empty and contain no "/". Other links name nothing.
func refOf(link string) (observation.Ref, bool) {
	for _, target := range []struct{ marker, kind string }{
		{pathDependencies + "/", kindDependency},
		{pathProducts + "/", kindProduct},
	} {
		i := strings.LastIndex(link, target.marker)
		if i < 0 {
			continue
		}
		id, err := url.PathUnescape(link[i+len(target.marker):])
		if err != nil || id == "" || strings.Contains(id, "/") {
			return observation.Ref{}, false
		}
		return observation.Ref{Kind: target.kind, ID: id}, true
	}
	return observation.Ref{}, false
}

// Observe reads readiness, dependencies, products and orders, in this order,
// and returns them as one snapshot stamped with now() after the last read. A
// failed read or a malformed item becomes a gap of the snapshot; only when
// every read fails does Observe return an error. The reads are not atomic:
// they can reflect different simulation ticks.
func (a *Adapter) Observe(ctx context.Context) (observation.Snapshot, error) {
	b := &build{observed: map[observation.Ref]bool{}, readOK: map[string]bool{}}
	var failed []error
	fail := func(path string, err error) {
		b.gap(path, "%s", err.Error())
		failed = append(failed, fmt.Errorf("read %s: %w", path, err))
	}

	var health healthDoc
	// Readiness down answers 503 with a full report: that is an observation.
	if err := a.read(ctx, pathReadiness, &health, http.StatusOK, http.StatusServiceUnavailable); err != nil {
		fail(pathReadiness, err)
	} else if health.Status == "" {
		fail(pathReadiness, errors.New("readiness without status"))
	} else {
		b.readOK[kindService] = true
		b.readOK[kindHealthCheck] = true
	}
	var deps dependencyListDoc
	if err := a.read(ctx, pathDependencies, &deps, http.StatusOK); err != nil {
		fail(pathDependencies, err)
	} else {
		b.readOK[kindDependency] = true
	}
	var products productListDoc
	if err := a.read(ctx, pathProducts, &products, http.StatusOK); err != nil {
		fail(pathProducts, err)
	} else {
		b.readOK[kindProduct] = true
	}
	var orders orderListDoc
	if err := a.read(ctx, pathOrders, &orders, http.StatusOK); err != nil {
		fail(pathOrders, err)
	} else {
		b.readOK[kindOrder] = true
	}
	if len(failed) == 4 {
		return observation.Snapshot{}, fmt.Errorf("adapter: nothing observed: %w", errors.Join(failed...))
	}

	// Entities first, so that every relation can check its target.
	var checks []checkDoc
	if b.readOK[kindService] {
		checks = mapHealth(b, health)
	}
	mapDependencies(b, deps)
	mapProducts(b, products)
	kept := mapOrders(b, orders)

	relateChecks(b, health.Status, checks)
	relateOrders(b, kept)

	snap, err := observation.NewSnapshot(a.now(), b.entities, b.relations, b.gaps)
	if err != nil {
		return observation.Snapshot{}, fmt.Errorf("adapter: %w", err)
	}
	return snap, nil
}

// read gets path, requires one of the accepted statuses and decodes the body
// into dst. Errors do not name the path; the caller does.
func (a *Adapter) read(ctx context.Context, path string, dst any, accepted ...int) error {
	doc, err := a.reader.Get(ctx, path)
	if err != nil {
		return err
	}
	if !slices.Contains(accepted, doc.Status) {
		var body errorDoc
		if doc.Decode(&body) == nil && body.Error.Code != "" {
			return fmt.Errorf("unexpected status %d: %s", doc.Status, body.Error.Code)
		}
		return fmt.Errorf("unexpected status %d", doc.Status)
	}
	return doc.Decode(dst)
}

// mapHealth maps the readiness report to the service entity followed by one
// entity per valid check, and returns the valid checks.
func mapHealth(b *build, h healthDoc) []checkDoc {
	b.add(observation.Entity{Ref: observation.Ref{Kind: kindService, ID: serviceID}, State: h.Status})
	var kept []checkDoc
	for i, c := range h.Checks {
		switch {
		case c.Name == "":
			b.gap(pathReadiness, "check %d without name", i)
			continue
		case c.Status == "":
			b.gap(pathReadiness, "check %d without status", i)
			continue
		}
		b.add(observation.Entity{
			Ref: observation.Ref{Kind: kindHealthCheck, ID: c.Name}, State: c.Status, Reason: c.Reason,
		})
		kept = append(kept, c)
	}
	return kept
}

// relateChecks relates the service to each check and each check to its
// causes. A check is a cause of the service state when its status equals the
// service status: the source documents the service status as the worst check
// status, so exactly these checks decide it.
func relateChecks(b *build, serviceStatus string, checks []checkDoc) {
	service := observation.Ref{Kind: kindService, ID: serviceID}
	for _, c := range checks {
		check := observation.Ref{Kind: kindHealthCheck, ID: c.Name}
		b.relate(pathReadiness, service, relationHasCheck, check, c.Status == serviceStatus)
		for _, link := range c.Causes {
			b.relateLink(pathReadiness, check, relationCausedBy, link, true)
		}
	}
}

func mapDependencies(b *build, deps dependencyListDoc) {
	for i, d := range deps.Dependencies {
		switch {
		case d.Name == "":
			b.gap(pathDependencies, "dependency %d without name", i)
			continue
		case d.Mode == "":
			b.gap(pathDependencies, "dependency %d without mode", i)
			continue
		}
		b.add(observation.Entity{Ref: observation.Ref{Kind: kindDependency, ID: d.Name}, State: d.Mode})
	}
}

func mapProducts(b *build, products productListDoc) {
	for i, p := range products.Products {
		if p.SKU == "" {
			b.gap(pathProducts, "product %d without sku", i)
			continue
		}
		b.add(observation.Entity{
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
}

// mapHistory maps the history of order i, oldest first. An entry without a
// target status is recorded as a gap and the order is skipped (false).
func mapHistory(b *build, i int, docs []transitionDoc) ([]observation.Transition, bool) {
	var history []observation.Transition
	for j, t := range docs {
		if t.To == "" {
			b.gap(pathOrders, "order %d history entry %d without to", i, j)
			return nil, false
		}
		history = append(history, observation.Transition{
			From: t.From, To: t.To, At: fmt.Sprintf("tick %d", t.AtTick), Reason: t.Reason,
		})
	}
	return history, true
}

// mapOrders maps every valid order and returns the valid orders.
func mapOrders(b *build, orders orderListDoc) []orderDoc {
	var kept []orderDoc
	for i, o := range orders.Orders {
		switch {
		case o.ID == "":
			b.gap(pathOrders, "order %d without id", i)
			continue
		case o.SKU == "":
			b.gap(pathOrders, "order %d without sku", i)
			continue
		case o.Status == "":
			b.gap(pathOrders, "order %d without status", i)
			continue
		}
		history, ok := mapHistory(b, i, o.History)
		if !ok {
			continue
		}
		// The last transition says why the order is in its state. For a failed
		// order it equals the source field failure_reason by construction.
		var reason string
		if len(history) > 0 {
			reason = history[len(history)-1].Reason
		}
		b.add(observation.Entity{
			Ref: observation.Ref{Kind: kindOrder, ID: o.ID}, State: o.Status, Reason: reason, History: history,
			Attributes: []observation.Attribute{
				{Name: "quantity", Value: strconv.Itoa(o.Quantity)},
				{Name: "total_cents", Value: strconv.FormatInt(o.TotalCents, 10)},
				{Name: "channel", Value: o.Channel},
				{Name: "placed_at_tick", Value: strconv.FormatUint(o.PlacedAtTick, 10)},
				{Name: "updated_at_tick", Value: strconv.FormatUint(o.UpdatedAtTick, 10)},
			},
		})
		kept = append(kept, o)
	}
	return kept
}

// relateOrders relates each order to its product, to the cause of its last
// transition and to the dependency it waits on. The last transition decided
// the current state, and an open order stays in its state until the
// dependency it waits on answers, so both are causes.
func relateOrders(b *build, orders []orderDoc) {
	for _, o := range orders {
		order := observation.Ref{Kind: kindOrder, ID: o.ID}
		b.relate(pathOrders, order, relationForProduct, observation.Ref{Kind: kindProduct, ID: o.SKU}, false)
		if n := len(o.History); n > 0 && o.History[n-1].Cause != "" {
			b.relateLink(pathOrders, order, relationCausedBy, o.History[n-1].Cause, true)
		}
		if o.Links.WaitingOn != "" {
			b.relateLink(pathOrders, order, relationWaitsOn, o.Links.WaitingOn, true)
		}
	}
}
