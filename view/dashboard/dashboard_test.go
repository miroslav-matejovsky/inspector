package dashboard_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/view/chart"
	"github.com/miroslav-matejovsky/inspector/view/dashboard"
)

func render(t *testing.T, m *model.Model) string {
	t.Helper()
	out, err := dashboard.Render(m, "/model")
	require.NoError(t, err)
	return string(out)
}

func down(id string) model.Entity {
	return model.Entity{ID: id, Kind: "k", State: model.State{Health: model.HealthDown}}
}

// attentionRows returns the rows of the attention table.
func attentionRows(out string) []string {
	var rows []string
	for r := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(r, `<tr class="health-`) {
			rows = append(rows, r)
		}
	}
	return rows
}

func TestRenderShowsHealthChart(t *testing.T) {
	m := model.New([]model.Entity{down("a"), {ID: "b", Kind: "l"}}, nil)
	want, err := chart.HealthByKind(m)
	require.NoError(t, err)

	require.Contains(t, render(t, m), string(want))
}

func TestRenderListsDownBeforeDegraded(t *testing.T) {
	m := model.New([]model.Entity{
		{ID: "d1", Kind: "k", State: model.State{Health: model.HealthDegraded}},
		{ID: "x", Kind: "k", State: model.State{Health: model.HealthOK}},
		down("d2"),
		{ID: "u", Kind: "k"},
	}, nil)

	rows := attentionRows(render(t, m))

	require.Len(t, rows, 2)
	require.Contains(t, rows[0], ">d2</a>")
	require.Contains(t, rows[1], ">d1</a>")
}

func TestRenderLinksAttentionToEntityPage(t *testing.T) {
	out := render(t, model.New([]model.Entity{down("/deps/pg")}, nil))

	require.Contains(t, out, `href="/model?entity=%2Fdeps%2Fpg"`)
}

func TestRenderShowsStateAndReason(t *testing.T) {
	e := down("pg")
	e.State.Value, e.State.Reason = "outage", "calls fail"

	rows := attentionRows(render(t, model.New([]model.Entity{e}, nil)))

	require.Len(t, rows, 1)
	require.Contains(t, rows[0], "<td>outage</td><td>calls fail</td>")
}

func TestRenderCountsAttention(t *testing.T) {
	out := render(t, model.New([]model.Entity{down("a"), down("b"), down("c")}, nil))

	require.Contains(t, out, `Attention <span class="count">3</span>`)
}

func TestRenderLimitsAttention(t *testing.T) {
	var es []model.Entity
	for i := range 51 {
		es = append(es, down(fmt.Sprint(i)))
	}

	out := render(t, model.New(es, nil))

	require.Len(t, attentionRows(out), 50)
	require.Contains(t, out, `Attention <span class="count">51</span>`)
	require.Contains(t, out, `<p class="note">1 more not shown</p>`)
}

func TestRenderShowsNoAttentionNote(t *testing.T) {
	out := render(t, model.New([]model.Entity{{ID: "x", Kind: "k", State: model.State{Health: model.HealthOK}}}, nil))

	require.Contains(t, out, `<p class="note">no entity is degraded or down</p>`)
}

func TestRenderListsIssues(t *testing.T) {
	out := render(t, model.New(nil, []model.Issue{{Target: "products", Error: "interpret: status 503"}}))

	require.Contains(t, out, `<span class="target">products</span> <span class="error">interpret: status 503</span>`)
}

func TestRenderOmitsIssuesWhenNone(t *testing.T) {
	out := render(t, model.New(nil, nil))

	require.NotContains(t, out, `class="issues"`)
}

func TestRenderEscapesValues(t *testing.T) {
	e := down("x")
	e.State.Reason = "<i>x</i>"

	out := render(t, model.New([]model.Entity{e}, nil))

	require.Contains(t, out, "&lt;i&gt;x&lt;/i&gt;")
	require.NotContains(t, out, "<i>")
}

func TestRenderMarksRoot(t *testing.T) {
	out := render(t, model.New(nil, nil))

	require.True(t, strings.HasPrefix(strings.TrimSpace(out), `<div class="view view-dashboard">`), out)
}

func TestStylesStyleDashboard(t *testing.T) {
	require.Contains(t, string(dashboard.Styles), ".view-dashboard")
	require.Contains(t, string(dashboard.Styles), "grid-template-columns: repeat(2, minmax(0, 1fr))")
}
