package workbench_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

// allFlags returns every required flag with a valid value, in a stable order.
func allFlags() [][2]string {
	return [][2]string{
		{"addr", "localhost:8080"},
		{"inspected-seed", "42"},
		{"inspected-orders-per-tick", "1"},
		{"inspected-tick-interval", "1s"},
		{"inspected-request-timeout", "2s"},
		{"inspector-source-timeout", "2s"},
	}
}

// args builds an argument list from flags, skipping the flag named skip and
// replacing values with those in override.
func args(skip string, override map[string]string) []string {
	var out []string
	for _, f := range allFlags() {
		if f[0] == skip {
			continue
		}
		value := f[1]
		if v, ok := override[f[0]]; ok {
			value = v
		}
		out = append(out, "-"+f[0], value)
	}
	return out
}

func TestParseConfig(t *testing.T) {
	got, err := workbench.ParseConfig(args("", nil), io.Discard)

	require.NoError(t, err)
	require.Equal(t, workbench.Config{
		Addr: "localhost:8080",
		Inspected: inspected.Config{
			Seed: 42, OrdersPerTick: 1, TickInterval: time.Second, RequestTimeout: 2 * time.Second,
		},
		InspectorSourceTimeout: 2 * time.Second,
	}, got)
}

func TestParseConfigWithoutFlags(t *testing.T) {
	_, err := workbench.ParseConfig(nil, io.Discard)

	require.ErrorContains(t, err, "missing required flags: -addr, -inspected-orders-per-tick, "+
		"-inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -inspector-source-timeout")
}

func TestParseConfigRequiresEveryFlag(t *testing.T) {
	for _, f := range allFlags() {
		t.Run(f[0], func(t *testing.T) {
			_, err := workbench.ParseConfig(args(f[0], nil), io.Discard)

			require.ErrorContains(t, err, "missing required flags")
			require.ErrorContains(t, err, "-"+f[0])
		})
	}
}

func TestParseConfigAcceptsZeroSeedAndTraffic(t *testing.T) {
	got, err := workbench.ParseConfig(
		args("", map[string]string{"inspected-seed": "0", "inspected-orders-per-tick": "0"}), io.Discard)

	require.NoError(t, err)
	require.Equal(t, uint64(0), got.Inspected.Seed)
	require.Equal(t, 0, got.Inspected.OrdersPerTick)
}

func TestParseConfigRejectsInvalidValues(t *testing.T) {
	tests := map[string]map[string]string{
		"empty addr":                {"addr": ""},
		"zero tick interval":        {"inspected-tick-interval": "0s"},
		"zero request timeout":      {"inspected-request-timeout": "0s"},
		"too many orders":           {"inspected-orders-per-tick": "101"},
		"negative seed":             {"inspected-seed": "-1"},
		"unparsable interval":       {"inspected-tick-interval": "abc"},
		"negative orders":           {"inspected-orders-per-tick": "-1"},
		"unparsable seed":           {"inspected-seed": "x"},
		"unparsable timeout":        {"inspected-request-timeout": "x"},
		"unparsable orders":         {"inspected-orders-per-tick": "x"},
		"negative tick interval":    {"inspected-tick-interval": "-1s"},
		"zero source timeout":       {"inspector-source-timeout": "0s"},
		"negative source timeout":   {"inspector-source-timeout": "-1s"},
		"unparsable source timeout": {"inspector-source-timeout": "x"},
	}
	for name, override := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := workbench.ParseConfig(args("", override), io.Discard)

			require.Error(t, err)
		})
	}
}

func TestParseConfigWritesUsageToOutput(t *testing.T) {
	var out strings.Builder

	_, err := workbench.ParseConfig([]string{"-bogus"}, &out)

	require.Error(t, err)
	require.Contains(t, out.String(), "-inspected-seed")
}
