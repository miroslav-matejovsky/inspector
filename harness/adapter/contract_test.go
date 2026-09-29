package adapter_test

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
	"github.com/miroslav-matejovsky/inspector/observation"
)

// TestObserveInspectedService runs the adapter against the real inspected
// handler, so a drift of the JSON contract fails here.
func TestObserveInspectedService(t *testing.T) {
	app, err := inspected.New(inspected.Config{
		Seed: 1, OrdersPerTick: 0, TickInterval: time.Hour, RequestTimeout: time.Second,
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

	resp, err := srv.Client().Post(srv.URL+inspected.PathPrefix+"/api/orders", "application/json",
		strings.NewReader(`{"sku":"sku-001","quantity":1}`))
	require.NoError(t, err)
	var placed struct{ ID string }
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&placed))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	s, err := newAdapter(t, srv.URL+inspected.PathPrefix, srv.Client()).Observe(context.Background())
	require.NoError(t, err)

	byKind := map[string][]observation.Entity{}
	for _, e := range s.Entities() {
		byKind[e.Ref.Kind] = append(byKind[e.Ref.Kind], e)
	}
	states := func(kind string) map[string]string {
		out := map[string]string{}
		for _, e := range byKind[kind] {
			out[e.Ref.ID] = e.State
		}
		return out
	}
	require.Equal(t, map[string]string{"inspected": "up"}, states("service"))
	require.Equal(t, map[string]string{"payment-gateway": "up", "warehouse": "up", "inventory": "up"},
		states("health_check"))
	require.Equal(t, map[string]string{"sku-001": "", "sku-002": "", "sku-003": "", "sku-004": "", "sku-005": ""},
		states("product"))
	require.Equal(t, map[string]string{placed.ID: "pending"}, states("order"))

	product, ok := s.Entity(ref("product", "sku-001"))
	require.True(t, ok)
	require.Contains(t, product.Attributes, observation.Attribute{Name: "stock", Value: "40"})
	require.Contains(t, s.Relations(), observation.Relation{
		From: ref("order", placed.ID), Kind: "for_product", To: ref("product", "sku-001"),
	})
}
