package fulfillment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

func TestParseOrderStatus(t *testing.T) {
	valid := []fulfillment.OrderStatus{
		fulfillment.StatusPending,
		fulfillment.StatusPaid,
		fulfillment.StatusShipped,
		fulfillment.StatusFailed,
	}
	for _, want := range valid {
		got, err := fulfillment.ParseOrderStatus(string(want))
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	for _, in := range []string{"", "PAID", "unknown"} {
		_, err := fulfillment.ParseOrderStatus(in)
		require.ErrorIs(t, err, fulfillment.ErrInvalidOrderStatus, in)
	}
}

func TestParseDependencyMode(t *testing.T) {
	valid := []fulfillment.DependencyMode{
		fulfillment.ModeHealthy,
		fulfillment.ModeSlow,
		fulfillment.ModeOutage,
	}
	for _, want := range valid {
		got, err := fulfillment.ParseDependencyMode(string(want))
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	for _, in := range []string{"", "down"} {
		_, err := fulfillment.ParseDependencyMode(in)
		require.ErrorIs(t, err, fulfillment.ErrInvalidDependencyMode, in)
	}
}

func TestAllowedTransitions(t *testing.T) {
	want := []fulfillment.TransitionRule{
		{From: fulfillment.StatusPending, To: fulfillment.StatusPaid, Reason: fulfillment.ReasonPaymentAuthorized},
		{From: fulfillment.StatusPending, To: fulfillment.StatusFailed, Reason: fulfillment.ReasonPaymentDeclined},
		{From: fulfillment.StatusPending, To: fulfillment.StatusFailed, Reason: fulfillment.ReasonPaymentGatewayUnavailable},
		{From: fulfillment.StatusPaid, To: fulfillment.StatusShipped, Reason: fulfillment.ReasonShipped},
		{From: fulfillment.StatusPaid, To: fulfillment.StatusFailed, Reason: fulfillment.ReasonOutOfStock},
		{From: fulfillment.StatusPaid, To: fulfillment.StatusFailed, Reason: fulfillment.ReasonWarehouseUnavailable},
	}
	require.Equal(t, want, fulfillment.AllowedTransitions())
}

func TestEnumerations(t *testing.T) {
	require.Equal(t, []fulfillment.Channel{fulfillment.ChannelAPI, fulfillment.ChannelSimulation}, fulfillment.Channels())
	require.Equal(t,
		[]fulfillment.DependencyName{fulfillment.DependencyPaymentGateway, fulfillment.DependencyWarehouse},
		fulfillment.DependencyNames())
	require.Equal(t,
		[]fulfillment.DependencyMode{fulfillment.ModeHealthy, fulfillment.ModeSlow, fulfillment.ModeOutage},
		fulfillment.DependencyModes())
}

func TestEnumerationsReturnNewSlices(t *testing.T) {
	fulfillment.Channels()[0] = "x"
	fulfillment.DependencyNames()[0] = "x"
	fulfillment.DependencyModes()[0] = "x"
	fulfillment.AllowedTransitions()[0].Reason = "x"

	require.Equal(t, fulfillment.ChannelAPI, fulfillment.Channels()[0])
	require.Equal(t, fulfillment.DependencyPaymentGateway, fulfillment.DependencyNames()[0])
	require.Equal(t, fulfillment.ModeHealthy, fulfillment.DependencyModes()[0])
	require.Equal(t, fulfillment.ReasonPaymentAuthorized, fulfillment.AllowedTransitions()[0].Reason)
}

func TestTerminal(t *testing.T) {
	require.False(t, fulfillment.StatusPending.Terminal())
	require.False(t, fulfillment.StatusPaid.Terminal())
	require.True(t, fulfillment.StatusShipped.Terminal())
	require.True(t, fulfillment.StatusFailed.Terminal())
}

func TestStandardCatalog(t *testing.T) {
	catalog := fulfillment.StandardCatalog()
	require.Len(t, catalog, 5)
	for i, p := range catalog {
		require.Equal(t, fulfillment.SKU([]string{"sku-001", "sku-002", "sku-003", "sku-004", "sku-005"}[i]), p.SKU)
	}

	_, err := fulfillment.NewState(catalog)
	require.NoError(t, err)

	catalog[0].Stock = 0
	require.Equal(t, 40, fulfillment.StandardCatalog()[0].Stock)
}
