package workbench_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

// Scenarios of the review in dev/assessments/review, pinned end to end
// through the workbench handler.

// unavailableScenario serves scenario P2: an inspected app whose simulation
// never runs, and the inspector reading it. It returns the workbench handler.
func unavailableScenario(t *testing.T) http.Handler {
	t.Helper()
	app, err := inspected.New(inspected.Config{
		Seed: 1, OrdersPerTick: 0, TickInterval: time.Hour, RequestTimeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	// Run is not called: every request to the simulation times out.
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)
	h, err := workbench.InspectorHandler(srv.URL+inspected.PathPrefix, 2*time.Second)
	require.NoError(t, err)
	return workbench.Handler(app.Handler(), h)
}

// view gets path from h, requires 200 and decodes the JSON body into T.
func view[T any](t *testing.T, h http.Handler, path string) T {
	t.Helper()
	rec := get(h, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}

type gapBody struct{ Source, Error string }

type kindBody struct {
	Kind  string
	Count int
}

type overviewBody struct {
	Kinds []kindBody
	Gaps  []gapBody
}

type attributeBody struct{ Name, Value string }

type transitionBody struct{ From, To, At, Reason string }

type entityBody struct {
	Kind, ID, State, Reason string
	Attributes              []attributeBody
	History                 []transitionBody
}

type refBody struct{ Kind, ID string }

type relatedBody struct {
	Relation, Direction string
	Cause               bool
	Target              refBody
}

type detailBody struct {
	Entity  entityBody
	Related []relatedBody
}

// outageScenario serves scenario P1 and returns the workbench handler: seed
// 42, 5 healthy ticks, payment-gateway outage, 10 more ticks.
func outageScenario(t *testing.T) http.Handler {
	t.Helper()
	app, err := inspected.New(inspected.Config{
		Seed: 42, OrdersPerTick: 1, TickInterval: time.Hour, RequestTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	srv := httptest.NewServer(app.Handler())
	t.Cleanup(srv.Close)

	for _, step := range []struct{ method, path, body string }{
		{http.MethodPost, "/sim/advance", `{"ticks":5}`},
		{http.MethodPut, "/sim/dependencies/payment-gateway", `{"mode":"outage"}`},
		{http.MethodPost, "/sim/advance", `{"ticks":10}`},
	} {
		req, err := http.NewRequest(step.method, srv.URL+inspected.PathPrefix+step.path, strings.NewReader(step.body))
		require.NoError(t, err)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode, step.path)
	}

	h, err := workbench.InspectorHandler(srv.URL+inspected.PathPrefix, 5*time.Second)
	require.NoError(t, err)
	return workbench.Handler(app.Handler(), h)
}

func TestUnavailableScenarioKeepsReadiness(t *testing.T) {
	h := unavailableScenario(t)

	overview := view[overviewBody](t, h, "/inspector/")

	require.Equal(t, []kindBody{{"service", 1}, {"health_check", 1}}, overview.Kinds)
	require.Len(t, overview.Gaps, 3)
	for i, source := range []string{"/api/dependencies", "/api/products", "/api/orders"} {
		require.Equal(t, source, overview.Gaps[i].Source)
		require.Contains(t, overview.Gaps[i].Error, "simulation_unavailable")
	}

	check := view[detailBody](t, h, "/inspector/entities/health_check/simulation")
	require.Equal(t, "down", check.Entity.State)
	require.Equal(t, "context deadline exceeded", check.Entity.Reason)
	require.Empty(t, check.Entity.Attributes)
	require.NotNil(t, check.Entity.Attributes)
}

func TestOutageScenarioShowsHistory(t *testing.T) {
	h := outageScenario(t)

	order := view[detailBody](t, h, "/inspector/entities/order/ord-000005").Entity
	require.Equal(t, "failed", order.State)
	require.Equal(t, "payment_gateway_unavailable", order.Reason)
	require.Equal(t, []transitionBody{
		{To: "pending", At: "tick 5", Reason: "order_placed"},
		{From: "pending", To: "failed", At: "tick 8", Reason: "payment_gateway_unavailable"},
	}, order.History)
	var names []string
	for _, a := range order.Attributes {
		names = append(names, a.Name)
	}
	require.Equal(t, []string{"quantity", "total_cents", "channel", "placed_at_tick", "updated_at_tick"}, names)

	check := view[detailBody](t, h, "/inspector/entities/health_check/payment-gateway").Entity
	require.Equal(t, "down", check.State)
	require.Equal(t, "payment-gateway is in outage: calls fail", check.Reason)
}

func TestOutageScenarioConnectsCauses(t *testing.T) {
	h := outageScenario(t)
	gateway := refBody{"dependency", "payment-gateway"}

	order := view[detailBody](t, h, "/inspector/entities/order/ord-000005")
	require.Contains(t, order.Related, relatedBody{Relation: "caused_by", Direction: "outgoing", Cause: true, Target: gateway})

	dep := view[detailBody](t, h, "/inspector/entities/dependency/payment-gateway")
	require.Equal(t, "outage", dep.Entity.State)
	count := map[string]int{}
	for _, r := range dep.Related {
		require.Equal(t, "incoming", r.Direction)
		require.True(t, r.Cause)
		count[r.Relation+" "+r.Target.Kind]++
	}
	require.Equal(t, map[string]int{"caused_by health_check": 1, "caused_by order": 8, "waits_on order": 3}, count)

	service := view[detailBody](t, h, "/inspector/entities/service/inspected")
	causes := map[string]bool{}
	for _, r := range service.Related {
		require.Equal(t, "has_check", r.Relation)
		causes[r.Target.ID] = r.Cause
	}
	require.Equal(t, map[string]bool{"payment-gateway": true, "warehouse": false, "inventory": false}, causes)
}

type rootBody struct{ Kind, ID, State string }

type causeBody struct {
	Relation, Kind, ID, State, Reason string
	Repeated                          bool
	Causes                            []causeBody
}

type explanationBody struct {
	Gaps        []gapBody
	Explanation struct {
		Kind, ID, State, Reason string
		History                 []transitionBody
		Causes                  []causeBody
	}
	RootCauses []rootBody `json:"root_causes"`
}

func TestOutageScenarioExplainsService(t *testing.T) {
	h := outageScenario(t)

	got := view[explanationBody](t, h, "/inspector/entities/service/inspected/explanation")

	require.Equal(t, []rootBody{{"dependency", "payment-gateway", "outage"}}, got.RootCauses)
	require.Equal(t, []causeBody{{
		Relation: "has_check", Kind: "health_check", ID: "payment-gateway", State: "down",
		Reason: "payment-gateway is in outage: calls fail",
		Causes: []causeBody{{Relation: "caused_by", Kind: "dependency", ID: "payment-gateway", State: "outage", Causes: []causeBody{}}},
	}}, got.Explanation.Causes)
}

func TestOutageScenarioExplainsOrder(t *testing.T) {
	h := outageScenario(t)

	got := view[explanationBody](t, h, "/inspector/entities/order/ord-000005/explanation")

	require.Equal(t, "payment_gateway_unavailable", got.Explanation.Reason)
	require.Equal(t, []transitionBody{
		{To: "pending", At: "tick 5", Reason: "order_placed"},
		{From: "pending", To: "failed", At: "tick 8", Reason: "payment_gateway_unavailable"},
	}, got.Explanation.History)
	require.Equal(t, []rootBody{{"dependency", "payment-gateway", "outage"}}, got.RootCauses)
}

func TestUnavailableScenarioExplainsService(t *testing.T) {
	h := unavailableScenario(t)

	got := view[explanationBody](t, h, "/inspector/entities/service/inspected/explanation")

	require.Equal(t, []rootBody{{"health_check", "simulation", "down"}}, got.RootCauses)
	require.Len(t, got.Gaps, 3)
}
