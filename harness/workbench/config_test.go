package workbench_test

import (
	"io"
	"log/slog"
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
		{"log-dir", "logs"},
		{"log-level", "info"},
		{"inspected-seed", "42"},
		{"inspected-orders-per-tick", "1"},
		{"inspected-tick-interval", "1s"},
		{"inspected-request-timeout", "2s"},
		{"source-database", "data/source.db"},
		{"source-interval", "5s"},
		{"source-timeout", "2s"},
		{"source-retention", "10m"},
		{"source-max-body-bytes", "2097152"},
		{"source-target", "alpha=/a"},
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
		Addr:     "localhost:8080",
		LogDir:   "logs",
		LogLevel: slog.LevelInfo,
		Inspected: inspected.Config{
			Seed: 42, OrdersPerTick: 1, TickInterval: time.Second, RequestTimeout: 2 * time.Second,
		},
		Source: workbench.SourceConfig{
			DatabasePath: "data/source.db",
			Interval:     5 * time.Second,
			Timeout:      2 * time.Second,
			Retention:    10 * time.Minute,
			MaxBodyBytes: 2097152,
			Targets:      []workbench.SourceTarget{{Name: "alpha", Path: "/a"}},
		},
	}, got)
}

func TestParseConfigCollectsTargets(t *testing.T) {
	got, err := workbench.ParseConfig(
		append(args("source-target", nil), "-source-target", "alpha=/a", "-source-target", "beta=/b"), io.Discard)

	require.NoError(t, err)
	require.Equal(t, []workbench.SourceTarget{{Name: "alpha", Path: "/a"}, {Name: "beta", Path: "/b"}},
		got.Source.Targets)
}

func TestParseConfigRejectsDuplicateTargets(t *testing.T) {
	_, err := workbench.ParseConfig(
		append(args("source-target", nil), "-source-target", "alpha=/a", "-source-target", "alpha=/b"), io.Discard)

	require.ErrorContains(t, err, "duplicate name")
}

func TestParseConfigWithoutFlags(t *testing.T) {
	_, err := workbench.ParseConfig(nil, io.Discard)

	require.ErrorContains(t, err, "missing required flags: -addr, -inspected-orders-per-tick, "+
		"-inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -log-dir, -log-level, "+
		"-source-database, -source-interval, -source-max-body-bytes, -source-retention, -source-target, "+
		"-source-timeout")
}

func TestParseConfigAcceptsLogLevels(t *testing.T) {
	tests := map[string]slog.Level{"debug": slog.LevelDebug, "WARN": slog.LevelWarn, "error": slog.LevelError}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			got, err := workbench.ParseConfig(args("", map[string]string{"log-level": value}), io.Discard)

			require.NoError(t, err)
			require.Equal(t, want, got.LogLevel)
		})
	}
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
		"empty addr":             {"addr": ""},
		"empty log dir":          {"log-dir": ""},
		"unknown log level":      {"log-level": "loud"},
		"zero tick interval":     {"inspected-tick-interval": "0s"},
		"zero request timeout":   {"inspected-request-timeout": "0s"},
		"too many orders":        {"inspected-orders-per-tick": "101"},
		"negative seed":          {"inspected-seed": "-1"},
		"unparsable interval":    {"inspected-tick-interval": "abc"},
		"negative orders":        {"inspected-orders-per-tick": "-1"},
		"unparsable seed":        {"inspected-seed": "x"},
		"unparsable timeout":     {"inspected-request-timeout": "x"},
		"unparsable orders":      {"inspected-orders-per-tick": "x"},
		"negative tick interval": {"inspected-tick-interval": "-1s"},
		"target without =":       {"source-target": "alpha"},
		"relative target path":   {"source-target": "alpha=a"},
		"invalid target name":    {"source-target": "Alpha=/a"},
		"zero source interval":   {"source-interval": "0s"},
		"zero source timeout":    {"source-timeout": "0s"},
		"zero source retention":  {"source-retention": "0s"},
		"zero body limit":        {"source-max-body-bytes": "0"},
		"empty database path":    {"source-database": ""},
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
