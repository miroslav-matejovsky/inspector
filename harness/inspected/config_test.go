package inspected_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *inspected.Config)
		wantErr string // empty means valid
	}{
		{"valid", func(*inspected.Config) {}, ""},
		{"zero tick interval", func(c *inspected.Config) { c.TickInterval = 0 }, "tick interval"},
		{"zero request timeout", func(c *inspected.Config) { c.RequestTimeout = 0 }, "request timeout"},
		{"negative orders per tick", func(c *inspected.Config) { c.OrdersPerTick = -1 }, "orders per tick"},
		{"too many orders per tick", func(c *inspected.Config) { c.OrdersPerTick = 101 }, "orders per tick"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := inspected.Config{Seed: 1, OrdersPerTick: 1, TickInterval: time.Second, RequestTimeout: time.Second}
			tc.mutate(&cfg)

			err := cfg.Validate()

			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	_, err := inspected.New(inspected.Config{})

	require.Error(t, err)
}
