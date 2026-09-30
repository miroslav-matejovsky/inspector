package explorer_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/view/explorer"
)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

func index(t *testing.T, entities ...model.Entity) string {
	t.Helper()
	out, err := explorer.Index(model.New(entities, nil), "/model")
	require.NoError(t, err)
	return string(out)
}

func entity(t *testing.T, id string, entities ...model.Entity) string {
	t.Helper()
	out, err := explorer.Entity(model.New(entities, nil), id, "/model")
	require.NoError(t, err)
	return string(out)
}

// requireInOrder requires that every part is in s, each after the one before.
func requireInOrder(t *testing.T, s string, parts ...string) {
	t.Helper()
	at := 0
	for _, p := range parts {
		i := strings.Index(s[at:], p)
		require.GreaterOrEqual(t, i, 0, "%q not found after position %d in %s", p, at, s)
		at += i + len(p)
	}
}

func TestIndexGroupsEntitiesByKind(t *testing.T) {
	out := index(t,
		model.Entity{ID: "1", Kind: "b"},
		model.Entity{ID: "2", Kind: "a"},
		model.Entity{ID: "3", Kind: "b"},
	)

	requireInOrder(t, out,
		`<h3>b <span class="count">2</span></h3>`,
		`<h3>a <span class="count">1</span></h3>`,
	)
}

func TestIndexLinksEntityPages(t *testing.T) {
	out := index(t, model.Entity{ID: "/x/1", Kind: "k"})

	require.Contains(t, out, `href="/model?entity=%2Fx%2F1"`)
}

func TestIndexMarksHealth(t *testing.T) {
	out := index(t, model.Entity{ID: "x", Kind: "k", State: model.State{Health: model.HealthDown}})

	require.Contains(t, out, `<tr class="health-down">`)
	require.Contains(t, out, `<span class="badge">down</span>`)
}

func TestIndexShowsDashForEmptyState(t *testing.T) {
	out := index(t, model.Entity{ID: "x", Kind: "k"})

	require.Equal(t, 2, strings.Count(out, ">-<"))
}

func TestIndexCountsLinks(t *testing.T) {
	out := index(t,
		model.Entity{ID: "x", Kind: "k", Links: []model.Link{{Type: "t", To: "y"}, {Type: "u", To: "z"}}},
		model.Entity{ID: "w", Kind: "l", Links: []model.Link{{Type: "t", To: "x"}}},
	)

	requireInOrder(t, out, `href="/model?entity=x"`, `<td class="num">2</td>`, `<td class="num">1</td>`)
}

func TestIndexLimitsRows(t *testing.T) {
	var entities []model.Entity
	for i := range 101 {
		entities = append(entities, model.Entity{ID: fmt.Sprint(i), Kind: "k"})
	}

	out := index(t, entities...)

	require.Equal(t, 100, strings.Count(out, `<tr class="health-`))
	require.Contains(t, out, `<p class="note">1 more not shown</p>`)
}

func TestIndexShowsEmptyModel(t *testing.T) {
	out := index(t)

	require.Contains(t, out, `<p class="note">no entities</p>`)
}

func TestIndexEscapesValues(t *testing.T) {
	out := index(t, model.Entity{ID: "x", Kind: "k", Name: "<script>x</script>"})

	require.Contains(t, out, "&lt;script&gt;x&lt;/script&gt;")
	require.NotContains(t, out, "<script>")
}

func TestEntityShowsState(t *testing.T) {
	out := entity(t, "/d", model.Entity{
		ID: "/d", Kind: "dependency", Name: "payment",
		State: model.State{Value: "outage", Health: model.HealthDown, Reason: "calls fail"},
	})

	require.Contains(t, out, `<section class="entity health-down">`)
	requireInOrder(t, out, `<span class="kind">dependency</span>`, `<span class="name">payment</span>`,
		`<span class="badge">down</span>`, "outage", "calls fail")
}

func TestEntityShowsEvidence(t *testing.T) {
	out := entity(t, "x", model.Entity{
		ID: "x", Kind: "k",
		Evidence: model.Evidence{Target: "deps", URL: "http://example.test/deps", ObservedAt: t0},
	})

	require.Contains(t, out, "deps")
	require.Contains(t, out, `<a href="http://example.test/deps">`)
	require.Contains(t, out, "2026-09-30T01:02:03Z")
}

func TestEntityShowsProperties(t *testing.T) {
	out := entity(t, "x", model.Entity{ID: "x", Kind: "k", Properties: []model.Property{
		{Name: "stock", Value: "12"}, {Name: "capacity", Value: "40"},
	}})

	requireInOrder(t, out, "stock", "12", "capacity", "40")
}

func TestEntityLinksRelatedEntities(t *testing.T) {
	out := entity(t, "o",
		model.Entity{ID: "o", Kind: "order", Links: []model.Link{{Type: "waits on", To: "/d"}}},
		model.Entity{ID: "/d", Kind: "dependency", Name: "payment", State: model.State{Health: model.HealthDegraded}},
	)

	requireInOrder(t, out, "waits on", `href="/model?entity=%2Fd"`, `<span class="kind">dependency</span> payment`,
		`<span class="health">degraded</span>`)
}

func TestEntityMarksMissingEntities(t *testing.T) {
	out := entity(t, "o", model.Entity{ID: "o", Kind: "order", Links: []model.Link{{Type: "haunts", To: "ghost"}}})

	requireInOrder(t, out, "ghost", "not observed")
	require.NotRegexp(t, `href="[^"]*ghost`, out)
}

func TestEntityListsBacklinks(t *testing.T) {
	out := entity(t, "b",
		model.Entity{ID: "a", Kind: "k", Links: []model.Link{{Type: "x", To: "b"}}},
		model.Entity{ID: "b", Kind: "k"},
		model.Entity{ID: "c", Kind: "k", Links: []model.Link{{Type: "y", To: "b"}}},
	)

	requireInOrder(t, out, "Backlinks", `href="/model?entity=a"`, `href="/model?entity=c"`)
}

func TestEntityLimitsBacklinks(t *testing.T) {
	entities := []model.Entity{{ID: "b", Kind: "k"}}
	for i := range 101 {
		entities = append(entities, model.Entity{ID: fmt.Sprint(i), Kind: "k", Links: []model.Link{{Type: "x", To: "b"}}})
	}

	out := entity(t, "b", entities...)

	require.Equal(t, 100, strings.Count(out, `<a href="/model?entity=`))
	require.Contains(t, out, "1 more not shown")
}

func TestEntityShowsNoneForEmptySections(t *testing.T) {
	out := entity(t, "x", model.Entity{ID: "x", Kind: "k"})

	require.Equal(t, 3, strings.Count(out, `<p class="note">none</p>`))
}

func TestEntityRejectsUnknownID(t *testing.T) {
	_, err := explorer.Entity(model.New(nil, nil), "nope", "/model")

	require.ErrorContains(t, err, `entity "nope" is not in the model`)
}

func TestViewsMarkRoot(t *testing.T) {
	e := model.Entity{ID: "x", Kind: "k"}
	for _, out := range []string{index(t, e), entity(t, "x", e)} {
		require.True(t, strings.HasPrefix(strings.TrimSpace(out), `<div class="view view-explorer">`), out)
	}
}

func TestStylesStyleExplorer(t *testing.T) {
	require.Contains(t, string(explorer.Styles), ".view-explorer")
}
