package adapter

// Own copies of the JSON resources of the inspected service, limited to the
// fields the adapter maps. Unknown fields are ignored when decoding.

// errorDoc is the error body of the inspected service.
type errorDoc struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

type healthDoc struct {
	Status string     `json:"status"`
	Checks []checkDoc `json:"checks"`
}

type checkDoc struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Reason string   `json:"reason"`
	Causes []string `json:"causes"` // links of the resources that determine the status
}

type dependencyListDoc struct {
	Dependencies []dependencyDoc `json:"dependencies"`
}

type dependencyDoc struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type productListDoc struct {
	Products []productDoc `json:"products"`
}

type productDoc struct {
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	PriceCents   int64  `json:"price_cents"`
	Stock        int    `json:"stock"`
	Capacity     int    `json:"capacity"`
	ReorderPoint int    `json:"reorder_point"`
}

type orderListDoc struct {
	Orders []orderDoc `json:"orders"`
}

type orderDoc struct {
	ID            string          `json:"id"`
	SKU           string          `json:"sku"`
	Quantity      int             `json:"quantity"`
	TotalCents    int64           `json:"total_cents"`
	Channel       string          `json:"channel"`
	Status        string          `json:"status"`
	PlacedAtTick  uint64          `json:"placed_at_tick"`
	UpdatedAtTick uint64          `json:"updated_at_tick"`
	History       []transitionDoc `json:"history"`
	Links         orderLinksDoc   `json:"links"`
}

type orderLinksDoc struct {
	WaitingOn string `json:"waiting_on"` // the dependency an open order waits on
}

type transitionDoc struct {
	From   string `json:"from"`
	To     string `json:"to"`
	AtTick uint64 `json:"at_tick"`
	Reason string `json:"reason"`
	Cause  string `json:"cause"` // link of the resource that decided the transition
}
