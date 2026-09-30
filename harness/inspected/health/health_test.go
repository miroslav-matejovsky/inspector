package health_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
	"github.com/miroslav-matejovsky/inspector/harness/inspected/health"
)

// snapshot returns the snapshot of a fresh standard state with the given
// dependency modes applied.
func snapshot(t *testing.T, modes map[fulfillment.DependencyName]fulfillment.DependencyMode) fulfillment.Snapshot {
	t.Helper()
	s, err := fulfillment.NewState(fulfillment.StandardCatalog())
	require.NoError(t, err)
	for name, mode := range modes {
		require.NoError(t, s.SetDependencyMode(name, mode))
	}
	return s.Snapshot()
}

func TestEvaluateAllHealthy(t *testing.T) {
	got := health.Evaluate(snapshot(t, nil))

	require.Equal(t, health.Report{
		Status: health.StatusUp,
		Checks: []health.Check{
			{Name: "payment-gateway", Status: health.StatusUp, Dependency: fulfillment.DependencyPaymentGateway},
			{Name: "warehouse", Status: health.StatusUp, Dependency: fulfillment.DependencyWarehouse},
			{Name: health.CheckInventory, Status: health.StatusUp},
		},
	}, got)
}

func TestEvaluateSlowDependency(t *testing.T) {
	got := health.Evaluate(snapshot(t, map[fulfillment.DependencyName]fulfillment.DependencyMode{
		fulfillment.DependencyPaymentGateway: fulfillment.ModeSlow,
	}))

	require.Equal(t, health.StatusDegraded, got.Status)
	require.Equal(t, health.Check{
		Name: "payment-gateway", Status: health.StatusDegraded, Dependency: fulfillment.DependencyPaymentGateway,
		Reason: "payment-gateway is slow: calls take 3 ticks",
	}, got.Checks[0])
}

func TestEvaluateOutage(t *testing.T) {
	got := health.Evaluate(snapshot(t, map[fulfillment.DependencyName]fulfillment.DependencyMode{
		fulfillment.DependencyWarehouse: fulfillment.ModeOutage,
	}))

	require.Equal(t, health.StatusDown, got.Status)
	require.Equal(t, health.Check{
		Name: "warehouse", Status: health.StatusDown, Dependency: fulfillment.DependencyWarehouse,
		Reason: "warehouse is in outage: calls fail",
	}, got.Checks[1])
}

func TestEvaluateOutageWinsOverDegraded(t *testing.T) {
	got := health.Evaluate(snapshot(t, map[fulfillment.DependencyName]fulfillment.DependencyMode{
		fulfillment.DependencyPaymentGateway: fulfillment.ModeSlow,
		fulfillment.DependencyWarehouse:      fulfillment.ModeOutage,
	}))

	require.Equal(t, health.StatusDown, got.Status)
}

func TestEvaluateOutOfStock(t *testing.T) {
	snap := snapshot(t, nil)
	snap.Products[1].Stock = 0 // sku-002
	snap.Products[4].Stock = 0 // sku-005

	got := health.Evaluate(snap)

	require.Equal(t, health.StatusDegraded, got.Status)
	require.Equal(t, health.Check{
		Name: health.CheckInventory, Status: health.StatusDegraded, OutOfStock: []fulfillment.SKU{"sku-002", "sku-005"},
		Reason: "out of stock: sku-002, sku-005",
	}, got.Checks[2])
}

func TestEvaluateIsPure(t *testing.T) {
	snap := snapshot(t, map[fulfillment.DependencyName]fulfillment.DependencyMode{
		fulfillment.DependencyWarehouse: fulfillment.ModeSlow,
	})
	snap.Products[0].Stock = 0
	before := snap
	before.Products = append([]fulfillment.Product(nil), snap.Products...)
	before.Dependencies = append([]fulfillment.Dependency(nil), snap.Dependencies...)

	first := health.Evaluate(snap)
	second := health.Evaluate(snap)

	require.Equal(t, first, second)
	require.Equal(t, before, snap)
}
