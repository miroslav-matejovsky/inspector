package inspected

import (
	"net/url"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/health"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/simulation"
)

// JSON resources. They are the HTTP contract of the inspected service and are
// kept apart from the domain types on purpose.

type indexResource struct {
	Service     string            `json:"service"`
	Description string            `json:"description"`
	Links       map[string]string `json:"links"`
}

type liveResource struct {
	Status string `json:"status"`           // "up" or "down"
	Reason string `json:"reason,omitempty"` // the error when down
}

type healthResource struct {
	Status string          `json:"status"`
	Checks []checkResource `json:"checks"`
}

type checkResource struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type simulationResource struct {
	NowTick       uint64               `json:"now_tick"`
	ClockRunning  bool                 `json:"clock_running"`
	Seed          uint64               `json:"seed"`
	OrdersPerTick int                  `json:"orders_per_tick"`
	TickInterval  string               `json:"tick_interval"` // time.Duration.String()
	Dependencies  []dependencyResource `json:"dependencies"`
}

type dependencyResource struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type clockRequest struct {
	Running *bool `json:"running"` // required
}

type advanceRequest struct {
	Ticks int `json:"ticks"`
}

type dependencyModeRequest struct {
	Mode string `json:"mode"`
}

type errorResource struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func healthResourceOf(r health.Report) healthResource {
	checks := make([]checkResource, 0, len(r.Checks))
	for _, c := range r.Checks {
		checks = append(checks, checkResource{Name: c.Name, Status: string(c.Status), Reason: c.Reason})
	}
	return healthResource{Status: string(r.Status), Checks: checks}
}

func dependencyResourceOf(d fulfillment.Dependency) dependencyResource {
	return dependencyResource{Name: string(d.Name), Mode: string(d.Mode)}
}

func (a *App) simulationResourceOf(s simulation.Status) simulationResource {
	deps := make([]dependencyResource, 0, len(s.Dependencies))
	for _, d := range s.Dependencies {
		deps = append(deps, dependencyResourceOf(d))
	}
	return simulationResource{
		NowTick:       uint64(s.Now),
		ClockRunning:  s.ClockRunning,
		Seed:          a.cfg.Seed,
		OrdersPerTick: a.cfg.OrdersPerTick,
		TickInterval:  a.cfg.TickInterval.String(),
		Dependencies:  deps,
	}
}

type productResource struct {
	SKU          string       `json:"sku"`
	Name         string       `json:"name"`
	PriceCents   int64        `json:"price_cents"`
	Stock        int          `json:"stock"`
	Capacity     int          `json:"capacity"`
	ReorderPoint int          `json:"reorder_point"`
	Links        productLinks `json:"links"`
}

type productLinks struct {
	Self   string `json:"self"`   // the product
	Orders string `json:"orders"` // the orders of the product
}

type orderResource struct {
	ID            string               `json:"id"`
	SKU           string               `json:"sku"`
	Quantity      int                  `json:"quantity"`
	TotalCents    int64                `json:"total_cents"`
	Channel       string               `json:"channel"`
	Status        string               `json:"status"`
	PlacedAtTick  uint64               `json:"placed_at_tick"`
	UpdatedAtTick uint64               `json:"updated_at_tick"`
	FailureReason string               `json:"failure_reason,omitempty"`
	History       []transitionResource `json:"history"`
	Links         orderLinks           `json:"links"`
}

type transitionResource struct {
	From   string `json:"from,omitempty"` // omitted for the initial entry
	To     string `json:"to"`
	AtTick uint64 `json:"at_tick"`
	Reason string `json:"reason"`
}

type orderLinks struct {
	Self    string `json:"self"`    // the order
	Product string `json:"product"` // the ordered product
}

type productListResource struct {
	Products []productResource `json:"products"`
}

type orderListResource struct {
	Orders []orderResource `json:"orders"` // an empty list is [], never null
}

type placeOrderRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

func productURL(sku string) string {
	return PathPrefix + pathProducts + "/" + url.PathEscape(sku)
}

func orderURL(id string) string {
	return PathPrefix + pathOrders + "/" + url.PathEscape(id)
}

func productResourceOf(p fulfillment.Product) productResource {
	sku := string(p.SKU)
	return productResource{
		SKU:          sku,
		Name:         p.Name,
		PriceCents:   p.PriceCents,
		Stock:        p.Stock,
		Capacity:     p.Capacity,
		ReorderPoint: p.ReorderPoint,
		Links: productLinks{
			Self:   productURL(sku),
			Orders: PathPrefix + pathOrders + "?sku=" + url.QueryEscape(sku),
		},
	}
}

func orderResourceOf(o fulfillment.Order) orderResource {
	history := make([]transitionResource, 0, len(o.History))
	for _, t := range o.History {
		history = append(history, transitionResource{
			From: string(t.From), To: string(t.To), AtTick: uint64(t.At), Reason: string(t.Reason),
		})
	}
	id := string(o.ID)
	return orderResource{
		ID:            id,
		SKU:           string(o.SKU),
		Quantity:      o.Quantity,
		TotalCents:    o.TotalCents,
		Channel:       string(o.Channel),
		Status:        string(o.Status),
		PlacedAtTick:  uint64(o.PlacedAt),
		UpdatedAtTick: uint64(o.UpdatedAt),
		FailureReason: string(o.FailureReason),
		History:       history,
		Links:         orderLinks{Self: orderURL(id), Product: productURL(string(o.SKU))},
	}
}
