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
			{"name":"payment-gateway","status":"degraded","reason":"payment-gateway is slow: calls take 3 ticks",
				"causes":["/inspected/api/dependencies/payment-gateway"]},
			{"name":"inventory","status":"up"}]}`},
		"/api/dependencies": {http.StatusOK, `{"dependencies":[{"name":"payment-gateway","mode":"slow"},{"name":"warehouse","mode":"healthy"}]}`},
		"/api/products": {http.StatusOK, `{"products":[{"sku":"sku-001","name":"Mechanical Keyboard",
			"price_cents":8900,"stock":40,"capacity":40,"reorder_point":10,
			"links":{"self":"/inspected/api/products/sku-001"}}]}`},
		"/api/orders": {http.StatusOK, `{"orders":[{"id":"ord-000001","sku":"sku-001","quantity":2,
			"total_cents":17800,"channel":"api","status":"failed","placed_at_tick":1,"updated_at_tick":4,
			"failure_reason":"payment_gateway_unavailable","history":[{"to":"pending","at_tick":1,"reason":"order_placed"},{"from":"pending","to":"failed","at_tick":4,"reason":"payment_gateway_unavailable",
				"cause":"/inspected/api/dependencies/payment-gateway"}],
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
		{Ref: ref("health_check", "payment-gateway"), State: "degraded",
			Reason: "payment-gateway is slow: calls take 3 ticks"},
		{Ref: ref("health_check", "inventory"), State: "up"},
		{Ref: ref("dependency", "payment-gateway"), State: "slow"},
		{Ref: ref("dependency", "warehouse"), State: "healthy"},
		{Ref: ref("product", "sku-001"), Attributes: []observation.Attribute{
			{Name: "name", Value: "Mechanical Keyboard"},
			{Name: "price_cents", Value: "8900"},
			{Name: "stock", Value: "40"},
			{Name: "capacity", Value: "40"},
			{Name: "reorder_point", Value: "10"},
		}},
		{Ref: ref("order", "ord-000001"), State: "failed", Reason: "payment_gateway_unavailable",
			History: []observation.Transition{
				{To: "pending", At: "tick 1", Reason: "order_placed"},
				{From: "pending", To: "failed", At: "tick 4", Reason: "payment_gateway_unavailable"},
			},
			Attributes: []observation.Attribute{
				{Name: "quantity", Value: "2"},
				{Name: "total_cents", Value: "17800"},
				{Name: "channel", Value: "api"},
				{Name: "placed_at_tick", Value: "1"},
				{Name: "updated_at_tick", Value: "4"},
			}},
	}, s.Entities())
	require.Empty(t, s.Gaps())
}

func TestObserveMapsRelations(t *testing.T) {
	s, err := observe(t, defaultDocs())

	require.NoError(t, err)
	require.Equal(t, []observation.Relation{
		{From: ref("service", "inspected"), Kind: "has_check", To: ref("health_check", "payment-gateway"), Cause: true},
		{From: ref("health_check", "payment-gateway"), Kind: "caused_by", To: ref("dependency", "payment-gateway"), Cause: true},
		{From: ref("service", "inspected"), Kind: "has_check", To: ref("health_check", "inventory")},
		{From: ref("order", "ord-000001"), Kind: "for_product", To: ref("product", "sku-001")},
		{From: ref("order", "ord-000001"), Kind: "caused_by", To: ref("dependency", "payment-gateway"), Cause: true},
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

// refs returns the refs of the entities of s of the given kind, in order.
func refs(s observation.Snapshot, kind string) []string {
	var out []string
	for _, e := range s.Entities() {
		if e.Ref.Kind == kind {
			out = append(out, e.Ref.ID)
		}
	}
	return out
}

func TestObserveRecordsFailedRead(t *testing.T) {
	docs := defaultDocs()
	docs["/api/products"] = doc{http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"x"}}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []string{"inspected"}, refs(s, "service"))
	require.Equal(t, []string{"payment-gateway", "inventory"}, refs(s, "health_check"))
	require.Equal(t, []string{"ord-000001"}, refs(s, "order"))
	require.Empty(t, refs(s, "product"))
	for _, r := range s.Relations() {
		require.NotEqual(t, "for_product", r.Kind)
	}
	require.Equal(t, []observation.Gap{{Source: "/api/products", Error: "unexpected status 500: internal_error"}}, s.Gaps())
}

func TestObserveRecordsMalformedItems(t *testing.T) {
	tests := map[string]struct {
		path, body string
		want       observation.Gap
		kind       string
		kept       []string
	}{
		"check without name": {"/health/ready",
			`{"status":"up","checks":[{"status":"up"},{"name":"inventory","status":"up"}]}`,
			observation.Gap{Source: "/health/ready", Error: "check 0 without name"}, "health_check", []string{"inventory"}},
		"check without status": {"/health/ready",
			`{"status":"up","checks":[{"name":"inventory","status":"up"},{"name":"x"}]}`,
			observation.Gap{Source: "/health/ready", Error: "check 1 without status"}, "health_check", []string{"inventory"}},
		"dependency without name": {"/api/dependencies",
			`{"dependencies":[{"mode":"slow"},{"name":"payment-gateway","mode":"slow"}]}`,
			observation.Gap{Source: "/api/dependencies", Error: "dependency 0 without name"}, "dependency", []string{"payment-gateway"}},
		"dependency without mode": {"/api/dependencies",
			`{"dependencies":[{"name":"payment-gateway","mode":"slow"},{"name":"x"}]}`,
			observation.Gap{Source: "/api/dependencies", Error: "dependency 1 without mode"}, "dependency", []string{"payment-gateway"}},
		"product without sku": {"/api/products",
			`{"products":[{"name":"x"},{"sku":"sku-001"}]}`,
			observation.Gap{Source: "/api/products", Error: "product 0 without sku"}, "product", []string{"sku-001"}},
		"order without id": {"/api/orders",
			`{"orders":[{"sku":"sku-001","status":"pending"},{"id":"o2","sku":"sku-001","status":"pending"}]}`,
			observation.Gap{Source: "/api/orders", Error: "order 0 without id"}, "order", []string{"o2"}},
		"order without sku": {"/api/orders",
			`{"orders":[{"id":"o1","status":"pending"},{"id":"o2","sku":"sku-001","status":"pending"}]}`,
			observation.Gap{Source: "/api/orders", Error: "order 0 without sku"}, "order", []string{"o2"}},
		"order without status": {"/api/orders",
			`{"orders":[{"id":"o1","sku":"sku-001"},{"id":"o2","sku":"sku-001","status":"pending"}]}`,
			observation.Gap{Source: "/api/orders", Error: "order 0 without status"}, "order", []string{"o2"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			docs := defaultDocs()
			docs[tc.path] = doc{http.StatusOK, tc.body}

			s, err := observe(t, docs)

			require.NoError(t, err)
			require.Equal(t, []observation.Gap{tc.want}, s.Gaps())
			require.Equal(t, tc.kept, refs(s, tc.kind))
		})
	}
}

func TestObserveRecordsHistoryWithoutTarget(t *testing.T) {
	docs := defaultDocs()
	docs["/api/orders"] = doc{http.StatusOK, `{"orders":[{"id":"o1","sku":"sku-001","status":"failed",
		"history":[{"to":"pending","at_tick":1,"reason":"order_placed"},{"from":"pending","at_tick":2}]}]}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []observation.Gap{{Source: "/api/orders", Error: "order 0 history entry 1 without to"}}, s.Gaps())
	require.Empty(t, refs(s, "order"))
}

func TestObserveRecordsReadinessWithoutStatus(t *testing.T) {
	docs := defaultDocs()
	docs["/health/ready"] = doc{http.StatusOK, `{"checks":[{"name":"inventory","status":"up"}]}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []observation.Gap{{Source: "/health/ready", Error: "readiness without status"}}, s.Gaps())
	require.Empty(t, refs(s, "service"))
	require.Empty(t, refs(s, "health_check"))
	require.Equal(t, []string{"sku-001"}, refs(s, "product"))
	require.Equal(t, []string{"ord-000001"}, refs(s, "order"))
}

func TestObserveRecordsUnknownProduct(t *testing.T) {
	docs := defaultDocs()
	docs["/api/orders"] = doc{http.StatusOK, `{"orders":[{"id":"o1","sku":"sku-999","status":"pending"}]}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []string{"o1"}, refs(s, "order"))
	for _, r := range s.Relations() {
		require.NotEqual(t, ref("order", "o1"), r.From)
	}
	require.Equal(t, []observation.Gap{
		{Source: "/api/orders", Error: "order/o1: for_product target product/sku-999 not observed"},
	}, s.Gaps())
}

func TestObserveFailsWhenNothingIsObserved(t *testing.T) {
	srv, a := source(t, defaultDocs())
	srv.Close()

	_, err := a.Observe(context.Background())

	require.ErrorContains(t, err, "nothing observed")
	require.ErrorContains(t, err, "/health/ready")
	require.ErrorContains(t, err, "/api/dependencies")
	require.ErrorContains(t, err, "/api/products")
	require.ErrorContains(t, err, "/api/orders")
}

func TestNewRejectsMissingDependencies(t *testing.T) {
	reader, err := connectivity.NewHTTPReader("http://127.0.0.1:1/inspected", time.Second, http.DefaultClient)
	require.NoError(t, err)

	_, err = adapter.New(nil, time.Now)
	require.Error(t, err)

	_, err = adapter.New(reader, nil)
	require.Error(t, err)
}

// relationsFrom returns the relations of s that start at from.
func relationsFrom(s observation.Snapshot, from observation.Ref) []observation.Relation {
	var out []observation.Relation
	for _, r := range s.Relations() {
		if r.From == from {
			out = append(out, r)
		}
	}
	return out
}

// readiness returns a readiness document with one check.
func readiness(check string) doc {
	return doc{http.StatusOK, `{"status":"degraded","checks":[` + check + `]}`}
}

func TestObserveMapsWaitingOrder(t *testing.T) {
	docs := defaultDocs()
	docs["/api/orders"] = doc{http.StatusOK, `{"orders":[{"id":"o1","sku":"sku-001","status":"pending",
		"history":[{"to":"pending","at_tick":1,"reason":"order_placed"}],
		"links":{"waiting_on":"/inspected/api/dependencies/payment-gateway"}}]}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	order := ref("order", "o1")
	require.Equal(t, []observation.Relation{
		{From: order, Kind: "for_product", To: ref("product", "sku-001")},
		{From: order, Kind: "waits_on", To: ref("dependency", "payment-gateway"), Cause: true},
	}, relationsFrom(s, order))
}

func TestObserveRecordsUnknownLink(t *testing.T) {
	for _, link := range []string{
		"/elsewhere/x",
		"/inspected/api/products/a%2Fb",
		"/inspected/api/products/",
		"/inspected/api/orders/ord-000001",
	} {
		t.Run(link, func(t *testing.T) {
			docs := defaultDocs()
			docs["/health/ready"] = readiness(`{"name":"payment-gateway","status":"degraded","causes":["` + link + `"]}`)

			s, err := observe(t, docs)

			require.NoError(t, err)
			require.Equal(t, []observation.Gap{{
				Source: "/health/ready",
				Error:  `health_check/payment-gateway: unknown link "` + link + `"`,
			}}, s.Gaps())
			require.Empty(t, relationsFrom(s, ref("health_check", "payment-gateway")))
		})
	}
}

func TestObserveMapsInventoryCause(t *testing.T) {
	docs := defaultDocs()
	docs["/health/ready"] = readiness(`{"name":"inventory","status":"degraded","causes":["/inspected/api/products/sku-001"]}`)

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []observation.Relation{
		{From: ref("health_check", "inventory"), Kind: "caused_by", To: ref("product", "sku-001"), Cause: true},
	}, relationsFrom(s, ref("health_check", "inventory")))
}

func TestObserveRecordsMissingCauseTarget(t *testing.T) {
	docs := defaultDocs()
	docs["/health/ready"] = readiness(`{"name":"payment-gateway","status":"degraded","causes":["/inspected/api/dependencies/ghost"]}`)

	s, err := observe(t, docs)

	require.NoError(t, err)
	require.Equal(t, []observation.Gap{{
		Source: "/health/ready",
		Error:  "health_check/payment-gateway: caused_by target dependency/ghost not observed",
	}}, s.Gaps())
}

func TestObserveOmitsRelationsToFailedRead(t *testing.T) {
	docs := defaultDocs()
	docs["/api/dependencies"] = doc{http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"x"}}`}

	s, err := observe(t, docs)

	require.NoError(t, err)
	for _, r := range s.Relations() {
		require.NotEqual(t, "caused_by", r.Kind)
	}
	require.Len(t, s.Gaps(), 1)
	require.Equal(t, "/api/dependencies", s.Gaps()[0].Source)
}
