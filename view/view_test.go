package view_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/view"
)

func TestStylesDefineSharedPalette(t *testing.T) {
	for _, want := range []string{".view {", "--view-ok", "--view-degraded", "--view-bad", "--view-unknown", "prefers-color-scheme: dark"} {
		require.Contains(t, string(view.Styles), want)
	}
}

func TestStylesDefineHealthClasses(t *testing.T) {
	for _, want := range []string{".health-ok", ".health-degraded", ".health-down", ".health-unknown", ".view .health {", ".view .badge"} {
		require.Contains(t, string(view.Styles), want)
	}
}
