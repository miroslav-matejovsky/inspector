package inspected_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type transitionBody struct {
	From   string
	To     string
	AtTick uint64 `json:"at_tick"`
	Reason string
}

type orderBody struct {
	ID            string
	SKU           string
	Quantity      int
	TotalCents    int64 `json:"total_cents"`
	Channel       string
	Status        string
	PlacedAtTick  uint64 `json:"placed_at_tick"`
	UpdatedAtTick uint64 `json:"updated_at_tick"`
	FailureReason string `json:"failure_reason"`
	History       []transitionBody
	Links         struct{ Self, Product string }
}

type productBody struct {
	SKU          string
	Name         string
	PriceCents   int64 `json:"price_cents"`
	Stock        int
	Capacity     int
	ReorderPoint int `json:"reorder_point"`
	Links        struct{ Self, Orders string }
}

const (
	productsPath = prefix + "/api/products"
	ordersPath   = prefix + "/api/orders"
)

func TestListProducts(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, productsPath, "")

	require.Equal(t, http.StatusOK, rec.Code)
	body := decode[struct{ Products []json.RawMessage }](t, rec)
	require.Len(t, body.Products, 5)
	require.JSONEq(t, `{
		"sku":"sku-001","name":"Mechanical Keyboard","price_cents":8900,"stock":40,"capacity":40,"reorder_point":10,
		"links":{"self":"/inspected/api/products/sku-001","orders":"/inspected/api/orders?sku=sku-001"}
	}`, string(body.Products[0]))
}

func TestGetProduct(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()

	rec := serve(t, h, http.MethodGet, productsPath+"/sku-002", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "USB-C Dock", decode[productBody](t, rec).Name)

	rec = serve(t, h, http.MethodGet, productsPath+"/sku-999", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "product_not_found", decode[errorBody](t, rec).Error.Code)
}

func TestPlaceOrder(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodPost, ordersPath, `{"sku":"sku-001","quantity":2}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, prefix+"/api/orders/ord-000001", rec.Header().Get("Location"))
	got := decode[orderBody](t, rec)
	require.Equal(t, "pending", got.Status)
	require.Equal(t, int64(17800), got.TotalCents)
	require.Equal(t, "api", got.Channel)
	require.Equal(t, uint64(0), got.PlacedAtTick)
	require.Equal(t, []transitionBody{{To: "pending", AtTick: 0, Reason: "order_placed"}}, got.History)
	require.Equal(t, prefix+"/api/orders/ord-000001", got.Links.Self)
	require.Equal(t, prefix+"/api/products/sku-001", got.Links.Product)
}

func TestPlaceOrderRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
		code string
	}{
		{"unknown sku", `{"sku":"sku-999","quantity":1}`, http.StatusUnprocessableEntity, "unknown_product"},
		{"zero quantity", `{"sku":"sku-001","quantity":0}`, http.StatusUnprocessableEntity, "invalid_quantity"},
		{"quantity above max", `{"sku":"sku-001","quantity":11}`, http.StatusUnprocessableEntity, "invalid_quantity"},
		{"malformed json", `{`, http.StatusBadRequest, "invalid_request"},
		{"unknown field", `{"sku":"sku-001","quantity":1,"x":1}`, http.StatusBadRequest, "invalid_request"},
		{"empty body", ``, http.StatusBadRequest, "invalid_request"},
	}
	app := startApp(t, testConfig())
	h := app.Handler()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, h, http.MethodPost, ordersPath, tc.body)

			require.Equal(t, tc.want, rec.Code)
			require.Equal(t, tc.code, decode[errorBody](t, rec).Error.Code)
		})
	}

	rec := serve(t, h, http.MethodGet, ordersPath, "")
	require.Equal(t, "{\"orders\":[]}\n", rec.Body.String())
}

func TestGetOrder(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	serve(t, h, http.MethodPost, ordersPath, `{"sku":"sku-001","quantity":1}`)
	serve(t, h, http.MethodPost, prefix+"/sim/advance", `{"ticks":2}`)

	rec := serve(t, h, http.MethodGet, ordersPath+"/ord-000001", "")

	require.Equal(t, http.StatusOK, rec.Code)
	got := decode[orderBody](t, rec)
	require.Equal(t, "shipped", got.Status)
	require.Equal(t, []transitionBody{
		{To: "pending", AtTick: 0, Reason: "order_placed"},
		{From: "pending", To: "paid", AtTick: 1, Reason: "payment_authorized"},
		{From: "paid", To: "shipped", AtTick: 2, Reason: "shipped"},
	}, got.History)

	rec = serve(t, h, http.MethodGet, ordersPath+"/ord-999999", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "order_not_found", decode[errorBody](t, rec).Error.Code)
}

func TestListOrdersEmpty(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, ordersPath, "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"orders\":[]}\n", rec.Body.String())
}

func TestListOrdersFilters(t *testing.T) {
	app := startApp(t, testConfig())
	h := app.Handler()
	serve(t, h, http.MethodPost, ordersPath, `{"sku":"sku-001","quantity":2}`) // 17800, paid
	serve(t, h, http.MethodPost, ordersPath, `{"sku":"sku-003","quantity":2}`) // 65800, declined
	serve(t, h, http.MethodPost, prefix+"/sim/advance", `{"ticks":1}`)

	ids := func(query string) []string {
		rec := serve(t, h, http.MethodGet, ordersPath+query, "")
		require.Equal(t, http.StatusOK, rec.Code, query)
		var out []string
		for _, o := range decode[struct{ Orders []orderBody }](t, rec).Orders {
			out = append(out, o.ID)
		}
		return out
	}

	require.Equal(t, []string{"ord-000001", "ord-000002"}, ids(""))
	require.Equal(t, []string{"ord-000002"}, ids("?status=failed"))
	require.Equal(t, []string{"ord-000001"}, ids("?sku=sku-001"))
	require.Empty(t, ids("?status=paid&sku=sku-003"))
	require.Empty(t, ids("?sku=sku-999"))

	rec := serve(t, h, http.MethodGet, ordersPath+"?status=bogus", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_status", decode[errorBody](t, rec).Error.Code)
}

func TestIndexListsBusinessLinks(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodGet, prefix+"/", "")

	links := decode[struct{ Links map[string]string }](t, rec).Links
	require.Equal(t, productsPath, links["products"])
	require.Equal(t, ordersPath, links["orders"])
}

func TestOrdersMethodNotAllowed(t *testing.T) {
	app := startApp(t, testConfig())

	rec := serve(t, app.Handler(), http.MethodDelete, ordersPath, "")

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
