package workbench_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/workbench"
	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/source"
)

// interpretedTargets are the five modeled targets, name -> path, as in Taskfile.yml.
var interpretedTargets = [][2]string{
	{"index", "/inspected/"}, {"health_ready", "/inspected/health/ready"},
	{"dependencies", "/inspected/api/dependencies"}, {"products", "/inspected/api/products"},
	{"orders", "/inspected/api/orders"},
}

// read returns the summary that the Source would store for target name
// after reading path from h at t0.
func read(t *testing.T, h http.Handler, name, path string) source.TargetSummary {
	t.Helper()
	rec := get(h, path)
	return withSignal(name, path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
}

// withSignal returns the summary of target name at path with one read signal.
func withSignal(name, path string, status int, contentType, body string) source.TargetSummary {
	url := "http://workbench.test" + path
	return source.TargetSummary{
		Target:  source.Target{Name: name, URL: url},
		Signals: 1,
		Latest: &source.Signal{
			Target: name, URL: url, ObservedAt: t0,
			StatusCode: status, ContentType: contentType, Body: []byte(body),
		},
	}
}

// send serves method path with the JSON body by h and requires status want.
func send(t *testing.T, h http.Handler, method, path, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, want, rec.Code, rec.Body.String())
	return rec
}

// modelOf reads every interpreted target of h and builds the model.
func modelOf(t *testing.T, h http.Handler) *model.Model {
	t.Helper()
	var summaries []source.TargetSummary
	for _, target := range interpretedTargets {
		summaries = append(summaries, read(t, h, target[0], target[1]))
	}
	return workbench.InspectedModel(summaries)
}

// entity returns the entity id of m and requires that it exists.
func entity(t *testing.T, m *model.Model, id string) model.Entity {
	t.Helper()
	e, ok := m.Entity(id)
	require.True(t, ok, "no entity %q", id)
	return e
}

// property returns the value of the property name of e and requires that it exists.
func property(t *testing.T, e model.Entity, name string) string {
	t.Helper()
	for _, p := range e.Properties {
		if p.Name == name {
			return p.Value
		}
	}
	t.Fatalf("entity %q has no property %q", e.ID, name)
	return ""
}

func outage(t *testing.T, h http.Handler) {
	t.Helper()
	send(t, h, http.MethodPut, "/inspected/sim/dependencies/payment-gateway", `{"mode":"outage"}`, http.StatusOK)
}

// placeOrder places an order of sku-001 and returns its ID path.
func placeOrder(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := send(t, h, http.MethodPost, "/inspected/api/orders", `{"sku":"sku-001","quantity":1}`, http.StatusCreated)
	loc := rec.Header().Get("Location")
	require.NotEmpty(t, loc)
	return loc
}

func TestInspectedModelReadsService(t *testing.T) {
	m := modelOf(t, startInspected(t).Handler())

	e := entity(t, m, "/inspected/")
	require.Equal(t, "service", e.Kind)
	require.Equal(t, "inspected", e.Name)
	require.Equal(t, "Simulated order fulfillment service", property(t, e, "description"))
	require.Equal(t, []model.Link{{Type: "readiness", To: "/inspected/health/ready"}}, e.Links)
}

func TestInspectedModelReadsDependencies(t *testing.T) {
	h := startInspected(t).Handler()
	send(t, h, http.MethodPut, "/inspected/sim/dependencies/warehouse", `{"mode":"slow"}`, http.StatusOK)

	m := modelOf(t, h)

	payment := entity(t, m, "/inspected/api/dependencies/payment-gateway")
	require.Equal(t, "dependency", payment.Kind)
	require.Equal(t, model.State{Value: "healthy", Health: model.HealthOK}, payment.State)
	warehouse := entity(t, m, "/inspected/api/dependencies/warehouse")
	require.Equal(t, model.State{Value: "slow", Health: model.HealthDegraded}, warehouse.State)
}

func TestInspectedModelReadsReadiness(t *testing.T) {
	h := startInspected(t).Handler()
	outage(t, h)
	require.Equal(t, http.StatusServiceUnavailable, read(t, h, "health_ready", "/inspected/health/ready").Latest.StatusCode)

	m := modelOf(t, h)

	ready := entity(t, m, "/inspected/health/ready")
	require.Equal(t, "readiness", ready.Kind)
	require.Equal(t, "down", ready.State.Value)
	require.Equal(t, model.HealthDown, ready.State.Health)
	for _, check := range []string{"payment-gateway", "warehouse", "inventory"} {
		require.Contains(t, ready.Links, model.Link{Type: "check", To: "/inspected/health/ready#" + check})
	}
	check := entity(t, m, "/inspected/health/ready#payment-gateway")
	require.Equal(t, "check", check.Kind)
	require.Equal(t, model.HealthDown, check.State.Health)
	require.Equal(t, "payment-gateway is in outage: calls fail", check.State.Reason)
	require.Equal(t, []model.Link{{Type: "determined by", To: "/inspected/api/dependencies/payment-gateway"}}, check.Links)
}

func TestInspectedModelReadsProducts(t *testing.T) {
	m := modelOf(t, startInspected(t).Handler())

	e := entity(t, m, "/inspected/api/products/sku-001")
	require.Equal(t, "product", e.Kind)
	require.Equal(t, "sku-001", e.Name)
	var names []string
	for _, p := range e.Properties {
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"name", "price_cents", "stock", "capacity", "reorder_point"}, names)
	require.Equal(t, model.HealthUnknown, e.State.Health)
	require.Empty(t, e.Links)
}

func TestInspectedModelReadsOrders(t *testing.T) {
	h := startInspected(t).Handler()
	id := placeOrder(t, h)

	m := modelOf(t, h)

	e := entity(t, m, id)
	require.Equal(t, "order", e.Kind)
	require.Equal(t, "pending", e.State.Value)
	require.Equal(t, model.HealthOK, e.State.Health)
	require.Equal(t, []model.Link{
		{Type: "product", To: "/inspected/api/products/sku-001"},
		{Type: "waits on", To: "/inspected/api/dependencies/payment-gateway"},
	}, e.Links)
	require.Equal(t, "pending (order_placed)", property(t, e, "tick 0"))
}

func TestInspectedModelReadsFailedOrder(t *testing.T) {
	h := startInspected(t).Handler()
	outage(t, h)
	id := placeOrder(t, h)
	send(t, h, http.MethodPost, "/inspected/sim/advance", `{"ticks":3}`, http.StatusOK)

	m := modelOf(t, h)

	e := entity(t, m, id)
	require.Equal(t, "failed", e.State.Value)
	require.Equal(t, model.HealthDown, e.State.Health)
	require.Equal(t, "payment_gateway_unavailable", e.State.Reason)
	require.Contains(t, e.Links, model.Link{Type: "decided by", To: "/inspected/api/dependencies/payment-gateway"})
	for _, l := range e.Links {
		require.NotEqual(t, "waits on", l.Type)
	}
}

func TestInspectedModelListsOrdersNewestFirst(t *testing.T) {
	h := startInspected(t).Handler()
	placeOrder(t, h)
	second := placeOrder(t, h)

	m := modelOf(t, h)

	for _, e := range m.Entities() {
		if e.Kind == "order" {
			require.Equal(t, second, e.ID)
			return
		}
	}
	t.Fatal("no order in the model")
}

func TestInspectedModelLinksResolve(t *testing.T) {
	h := startInspected(t).Handler()
	outage(t, h)
	placeOrder(t, h)
	send(t, h, http.MethodPost, "/inspected/sim/advance", `{"ticks":3}`, http.StatusOK)

	m := modelOf(t, h)

	require.Empty(t, m.Issues())
	for _, e := range m.Entities() {
		for _, l := range e.Links {
			_, ok := m.Entity(l.To)
			require.True(t, ok, "entity %q: link %q to %q does not resolve", e.ID, l.Type, l.To)
		}
	}
}

func TestInspectedModelIgnoresOtherTargets(t *testing.T) {
	h := startInspected(t).Handler()

	m := workbench.InspectedModel([]source.TargetSummary{
		read(t, h, "health_live", "/inspected/health/live"),
		read(t, h, "metrics", "/inspected/metrics"),
	})

	require.Empty(t, m.Entities())
	require.Empty(t, m.Issues())
}

func TestInspectedModelReportsUnexpectedStatus(t *testing.T) {
	m := workbench.InspectedModel([]source.TargetSummary{
		withSignal("products", "/inspected/api/products", http.StatusServiceUnavailable, "application/json", `{}`),
	})

	require.Equal(t, []model.Issue{{Target: "products", Error: "interpret: status 503"}}, m.Issues())
}

func TestInspectedModelReportsInvalidBody(t *testing.T) {
	m := workbench.InspectedModel([]source.TargetSummary{
		withSignal("orders", "/inspected/api/orders", http.StatusOK, "application/json", `not json`),
	})

	require.Len(t, m.Issues(), 1)
	require.Equal(t, "orders", m.Issues()[0].Target)
	require.True(t, strings.HasPrefix(m.Issues()[0].Error, "interpret: decode body:"), m.Issues()[0].Error)
}

func TestInspectedModelReadsUnknownWordsAsUnknownHealth(t *testing.T) {
	m := workbench.InspectedModel([]source.TargetSummary{
		withSignal("dependencies", "/inspected/api/dependencies", http.StatusOK, "application/json",
			`{"dependencies":[{"name":"x","mode":"flaky","links":{"self":"/x"}}]}`),
	})

	e := entity(t, m, "/x")
	require.Equal(t, model.State{Value: "flaky", Health: model.HealthUnknown}, e.State)
}
