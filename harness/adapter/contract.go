package adapter

// Own copies of the JSON resources of the inspected service, limited to the
// fields the adapter maps. Unknown fields are ignored when decoding.

type healthDoc struct {
	Status string     `json:"status"`
	Checks []checkDoc `json:"checks"`
}

type checkDoc struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
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
	ID            string `json:"id"`
	SKU           string `json:"sku"`
	Quantity      int    `json:"quantity"`
	TotalCents    int64  `json:"total_cents"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	PlacedAtTick  uint64 `json:"placed_at_tick"`
	UpdatedAtTick uint64 `json:"updated_at_tick"`
	FailureReason string `json:"failure_reason"`
}
