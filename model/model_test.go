package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/model"
)

func TestNewKeepsEntitiesInOrder(t *testing.T) {
	m := model.New([]model.Entity{{ID: "c", Kind: "k"}, {ID: "a", Kind: "k"}, {ID: "b", Kind: "k"}}, nil)
	require.Equal(t, []string{"c", "a", "b"}, ids(m))
}

func TestNewDropsInvalidEntities(t *testing.T) {
	from := model.Evidence{Target: "t"}
	tests := []struct {
		name   string
		entity model.Entity
		issue  string
	}{
		{
			name:   "empty id",
			entity: model.Entity{Kind: "order", Name: "first", Evidence: from},
			issue:  `entity of kind "order" named "first": empty id`,
		},
		{
			name:   "empty kind",
			entity: model.Entity{ID: "x", Evidence: from},
			issue:  `entity "x": empty kind`,
		},
		{
			name:   "unknown health",
			entity: model.Entity{ID: "x", Kind: "k", State: model.State{Health: "fine"}, Evidence: from},
			issue:  `entity "x": unknown health "fine"`,
		},
		{
			name:   "link without type",
			entity: model.Entity{ID: "x", Kind: "k", Links: []model.Link{{Type: "t", To: "y"}, {To: "z"}}, Evidence: from},
			issue:  `entity "x": link 1: empty type or target`,
		},
		{
			name:   "link without target",
			entity: model.Entity{ID: "x", Kind: "k", Links: []model.Link{{Type: "t"}}, Evidence: from},
			issue:  `entity "x": link 0: empty type or target`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model.New([]model.Entity{tt.entity}, nil)
			require.Empty(t, m.Entities())
			require.Equal(t, []model.Issue{{Target: "t", Error: tt.issue}}, m.Issues())
		})
	}
}

func TestNewDropsDuplicateIDs(t *testing.T) {
	m := model.New([]model.Entity{
		{ID: "x", Kind: "k", Evidence: model.Evidence{Target: "a"}},
		{ID: "x", Kind: "k", Evidence: model.Evidence{Target: "b"}},
	}, nil)
	require.Len(t, m.Entities(), 1)
	require.Equal(t, "a", m.Entities()[0].Evidence.Target)
	require.Equal(t, []model.Issue{{Target: "b", Error: `entity "x": already read from target "a"`}}, m.Issues())
}

func TestNewKeepsGivenIssuesFirst(t *testing.T) {
	given := model.Issue{Target: "a", Error: "given"}
	m := model.New([]model.Entity{{ID: "x", Evidence: model.Evidence{Target: "b"}}}, []model.Issue{given})
	require.Equal(t, []model.Issue{given, {Target: "b", Error: `entity "x": empty kind`}}, m.Issues())
}

func TestNewReadsEmptyHealthAsUnknown(t *testing.T) {
	m := model.New([]model.Entity{{ID: "x", Kind: "k"}}, nil)
	require.Equal(t, model.HealthUnknown, m.Entities()[0].State.Health)
}

func TestNewNamesEntityByID(t *testing.T) {
	m := model.New([]model.Entity{{ID: "x", Kind: "k"}, {ID: "y", Kind: "k", Name: "why"}}, nil)
	require.Equal(t, "x", m.Entities()[0].Name)
	require.Equal(t, "why", m.Entities()[1].Name)
}

func TestModelFindsEntityByID(t *testing.T) {
	m := model.New([]model.Entity{{ID: "a", Kind: "k", Name: "A"}}, nil)
	e, ok := m.Entity("a")
	require.True(t, ok)
	require.Equal(t, "A", e.Name)
	e, ok = m.Entity("nope")
	require.False(t, ok)
	require.Equal(t, model.Entity{}, e)
}

func TestModelListsBacklinks(t *testing.T) {
	m := model.New([]model.Entity{
		{ID: "a", Kind: "k", Links: []model.Link{{Type: "x", To: "b"}}},
		{ID: "b", Kind: "k"},
		{ID: "c", Kind: "k", Links: []model.Link{{Type: "y", To: "b"}}},
	}, nil)
	require.Equal(t, []model.Backlink{{From: "a", Type: "x"}, {From: "c", Type: "y"}}, m.Backlinks("b"))
	require.Empty(t, m.Backlinks("a"))
}

func TestModelKeepsLinksToMissingEntities(t *testing.T) {
	m := model.New([]model.Entity{{ID: "a", Kind: "k", Links: []model.Link{{Type: "haunts", To: "ghost"}}}}, nil)
	a, ok := m.Entity("a")
	require.True(t, ok)
	require.Equal(t, []model.Link{{Type: "haunts", To: "ghost"}}, a.Links)
	_, ok = m.Entity("ghost")
	require.False(t, ok)
	require.Equal(t, []model.Backlink{{From: "a", Type: "haunts"}}, m.Backlinks("ghost"))
}
