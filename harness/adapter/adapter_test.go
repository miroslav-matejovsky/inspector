package adapter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/connectivity"
	"github.com/miroslav-matejovsky/inspector/harness/adapter"
	"github.com/miroslav-matejovsky/inspector/observation"
)

var observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// doc is one canned response of the fake inspected service.
type doc struct {
	status int
	body   string
}

// defaultDocs returns a degraded service with one product and one failed order.
func defaultDocs() map[string]doc {
	return map[string]doc{
		"/health/ready": {http.StatusOK, `{"status":"degraded","checks":[
			{"name":"payment-gateway","status":"degraded","reason":"payment-gateway is slow: calls take 3 ticks"},
			{"name":"inventory","status":"up"}]}`},
		"/api/products": {http.StatusOK, `{"products":[{"sku":"sku-001","name":"Mechanical Keyboard",
			"price_cents":8900,"stock":40,"capacity":40,"reorder_point":10,
			"links":{"self":"/inspected/api/products/sku-001"}}]}`},
		"/api/orders": {http.StatusOK, `{"orders":[{"id":"ord-000001","sku":"sku-001","quantity":2,
			"total_cents":17800,"channel":"api","status":"failed","placed_at_tick":1,"updated_at_tick":4,
			"failure_reason":"payment_gateway_unavailable","history":[],
			"links":{"self":"/inspected/api/orders/ord-000001"}}]}`},
	}
}

// source serves docs under /inspected and returns the server and an adapter
// reading from it with a fixed clock.
func source(t *testing.T, docs map[string]doc) (*httptest.Server, *adapter.Adapter) {
	t.Helper()
	mux := http.NewServeMux()
	for path, d := range docs {
		mux.HandleFunc("GET /inspected"+path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(d.status)
			_, _ = w.Write([]byte(d.body))
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, newAdapter(t, srv.URL+"/inspected", srv.Client())
}

func newAdapter(t *testing.T, base string, client *http.Client) *adapter.Adapter {
	t.Helper()
	reader, err := connectivity.NewHTTPReader(base, time.Second, client)
	require.NoError(t, err)
	a, err := adapter.New(reader, func() time.Time { return observedAt })
	require.NoError(t, err)
	return a
}

func observe(t *testing.T, docs map[string]doc) (observation.Snapshot, error) {
	t.Helper()
	_, a := source(t, docs)
	return a.Observe(context.Background())
}

func ref(kind, id string) observation.Ref {
	return observation.Ref{Kind: kind, ID: id}
}

func TestObserveMapsEntities(t *testing.T) {
	s, err := observe(t, defaultDocs())

	require.NoError(t, err)
	require.Equal(t, []observation.Entity{
		{Ref: ref("service", "inspected"), State: "degraded"},
		{Ref: ref("health_check", "payment-gateway"), State: "degraded", Attributes: []observation.Attribute{
			{Name: "reason", Value: "payment-gateway is slow: calls take 3 ticks"},
		}},
		{Ref: ref("health_check", "inventory"), State: "up"},
		{Ref: ref("product", "sku-001"), Attributes: []observation.Attribute{
			{Name: "name", Value: "Mechanical Keyboard"},
			{Name: "price_cents", Value: "8900"},
			{Name: "stock", Value: "40"},
			{Name: "capacity", Value: "40"},
			{Name: "reorder_point", Value: "10"},
		}},
		{Ref: ref("order", "ord-000001"), State: "failed", Attributes: []observation.Attribute{
			{Name: "quantity", Value: "2"},
			{Name: "total_cents", Value: "17800"},
			{Name: "channel", Value: "api"},
			{Name: "placed_at_tick", Value: "1"},
			{Name: "updated_at_tick", Value: "4"},
			{Name: "failure_reason", Value: "payment_gateway_unavailable"},
		}},
	}, s.Entities())
}

func TestObserveMapsRelations(t *testing.T) {
	s, err := observe(t, defaultDocs())

	require.NoError(t, err)
	require.Equal(t, []observation.Relation{
		{From: ref("service", "inspected"), Kind: "has_check", To: ref("health_check", "payment-gateway")},
		{From: ref("service", "inspected"), Kind: "has_check", To: ref("health_check", "inventory")},
		{From: ref("order", "ord-000001"), Kind: "for_product", To: ref("product", "sku-001")},
	}, s.Relations())
}

func TestObserveStampsTime(t *testing.T) {
	s, err := observe(t, defaultDocs())

	require.NoError(t, err)
	require.Equal(t, observedAt, s.ObservedAt())
}

func TestObserveAcceptsUnavailableReadiness(t *testing.T) {
	docs := defaultDocs()
	docs["/health/ready"] = doc{http.StatusServiceUnavailable, `{"status":"down","checks":[
		{"name":"warehouse","status":"down","reason":"warehouse is in outage: calls fail"}]}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	service, ok := s.Entity(ref("service", "inspected"))
	require.True(t, ok)
	require.Equal(t, "down", service.State)
}

func TestObserveRejectsUnexpectedStatus(t *testing.T) {
	docs := defaultDocs()
	docs["/api/products"] = doc{http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"x"}}`}

	_, err := observe(t, docs)

	require.ErrorContains(t, err, "/api/products")
	require.ErrorContains(t, err, "500")
}

func TestObserveRejectsMissingField(t *testing.T) {
	tests := map[string]struct {
		path, body, want string
	}{
		"readiness without status": {"/health/ready", `{"checks":[]}`, "readiness without status"},
		"check without name": {"/health/ready",
			`{"status":"up","checks":[{"status":"up"}]}`, "check 0 without name"},
		"product without sku": {"/api/products",
			`{"products":[{"name":"x"}]}`, "product 0 without sku"},
		"order without id": {"/api/orders",
			`{"orders":[{"sku":"sku-001","status":"pending"}]}`, "order 0 without id"},
		"order without sku": {"/api/orders",
			`{"orders":[{"id":"o1","status":"pending"}]}`, "order 0 without sku"},
		"order without status": {"/api/orders",
			`{"orders":[{"id":"o1","sku":"sku-001"}]}`, "order 0 without status"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			docs := defaultDocs()
			docs[tc.path] = doc{http.StatusOK, tc.body}

			_, err := observe(t, docs)

			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestObserveRejectsUnknownProduct(t *testing.T) {
	docs := defaultDocs()
	docs["/api/orders"] = doc{http.StatusOK, `{"orders":[{"id":"o1","sku":"sku-999","status":"pending"}]}`}

	_, err := observe(t, docs)

	require.ErrorIs(t, err, observation.ErrInvalidSnapshot)
}

func TestObserveFailsWhenSourceIsDown(t *testing.T) {
	srv, a := source(t, defaultDocs())
	srv.Close()

	_, err := a.Observe(context.Background())

	require.ErrorContains(t, err, "/health/ready")
}

func TestNewRejectsMissingDependencies(t *testing.T) {
	reader, err := connectivity.NewHTTPReader("http://127.0.0.1:1/inspected", time.Second, http.DefaultClient)
	require.NoError(t, err)

	_, err = adapter.New(nil, time.Now)
	require.Error(t, err)

	_, err = adapter.New(reader, nil)
	require.Error(t, err)
}
