package chart_test

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/view/chart"
)

var rectRe = regexp.MustCompile(`<rect class="health-(\w+)" x="[\d.]+" y="[\d.]+" width="([\d.]+)"[^>]*><title>([^:]*):`)

func healthByKind(t *testing.T, entities ...model.Entity) string {
	t.Helper()
	out, err := chart.HealthByKind(model.New(entities, nil))
	require.NoError(t, err)
	return string(out)
}

// entities returns n entities of kind with health h.
func entities(kind string, h model.Health, n int) []model.Entity {
	var out []model.Entity
	for i := range n {
		out = append(out, model.Entity{ID: fmt.Sprintf("%s-%s-%d", kind, h, i), Kind: kind, State: model.State{Health: h}})
	}
	return out
}

// widths returns the widths of the rects of every kind, in order.
func widths(t *testing.T, html string) map[string][]float64 {
	t.Helper()
	out := map[string][]float64{}
	for _, m := range rectRe.FindAllStringSubmatch(html, -1) {
		w, err := strconv.ParseFloat(m[2], 64)
		require.NoError(t, err)
		out[m[3]] = append(out[m[3]], w)
	}
	return out
}

func TestHealthByKindDrawsOneBarPerKind(t *testing.T) {
	out := healthByKind(t,
		model.Entity{ID: "1", Kind: "order"},
		model.Entity{ID: "2", Kind: "product"},
		model.Entity{ID: "3", Kind: "order"},
	)

	require.Less(t, strings.Index(out, ">order<"), strings.Index(out, ">product<"))
	require.Equal(t, 2, strings.Count(out, `class="label"`))
}

func TestHealthByKindSplitsBarsByHealth(t *testing.T) {
	out := healthByKind(t, append(entities("order", model.HealthOK, 2), entities("order", model.HealthDown, 1)...)...)

	require.Contains(t, out, "<title>order: 2 ok</title>")
	require.Contains(t, out, "<title>order: 1 down</title>")
	require.NotContains(t, out, `<rect class="health-degraded"`)
	require.NotContains(t, out, `<rect class="health-unknown"`)
}

func TestHealthByKindOrdersSegments(t *testing.T) {
	var es []model.Entity
	for _, h := range []model.Health{model.HealthUnknown, model.HealthDown, model.HealthDegraded, model.HealthOK} {
		es = append(es, entities("k", h, 1)...)
	}

	out := healthByKind(t, es...)

	var order []string
	for _, m := range rectRe.FindAllStringSubmatch(out, -1) {
		order = append(order, m[1])
	}
	require.Equal(t, []string{"ok", "degraded", "down", "unknown"}, order)
}

func TestHealthByKindFillsBars(t *testing.T) {
	var es []model.Entity
	es = append(es, entities("one", model.HealthOK, 1)...)
	es = append(es, entities("three", model.HealthOK, 1)...)
	es = append(es, entities("three", model.HealthDown, 2)...)
	es = append(es, entities("seven", model.HealthOK, 3)...)
	es = append(es, entities("seven", model.HealthDegraded, 2)...)
	es = append(es, entities("seven", model.HealthUnknown, 2)...)

	got := widths(t, healthByKind(t, es...))

	require.Len(t, got, 3)
	for kind, ws := range got {
		sum := 0.0
		for _, w := range ws {
			sum += w
		}
		require.LessOrEqual(t, math.Abs(sum-400), 0.05, "kind %s: widths %v", kind, ws)
	}
}

func TestHealthByKindShowsTotals(t *testing.T) {
	out := healthByKind(t, entities("k", model.HealthOK, 3)...)

	require.Regexp(t, `<text class="count" [^>]*>3</text>`, out)
}

func TestHealthByKindShowsLegend(t *testing.T) {
	out := healthByKind(t, entities("k", model.HealthOK, 1)...)

	for _, h := range []string{"ok", "degraded", "down", "unknown"} {
		require.Contains(t, out, `<span class="health-`+h+`">`+h+`</span>`)
	}
}

func TestHealthByKindShowsEmptyModel(t *testing.T) {
	out := healthByKind(t)

	require.Contains(t, out, `<p class="note">no entities</p>`)
	require.NotContains(t, out, "<svg")
}

func TestHealthByKindEscapesKinds(t *testing.T) {
	out := healthByKind(t, model.Entity{ID: "x", Kind: "<b>"})

	require.Contains(t, out, `class="label" x="0" y="16.00">&lt;b&gt;</text>`)
	require.Contains(t, out, "<title>&lt;b&gt;: 1 unknown</title>")
	require.NotContains(t, out, "<b>")
}

func TestHealthByKindMarksRoot(t *testing.T) {
	for _, out := range []string{healthByKind(t), healthByKind(t, entities("k", model.HealthOK, 1)...)} {
		require.True(t, strings.HasPrefix(strings.TrimSpace(out), `<div class="view view-chart">`), out)
	}
}

func TestStylesColorBarsByHealth(t *testing.T) {
	require.Contains(t, string(chart.Styles), ".view-chart rect { fill: var(--view-health); }")
}
