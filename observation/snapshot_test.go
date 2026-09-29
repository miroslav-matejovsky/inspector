package observation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/observation"
)

var observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

var (
	b1 = observation.Entity{
		Ref: observation.Ref{Kind: "book", ID: "b1"}, State: "available",
		Attributes: []observation.Attribute{{Name: "title", Value: "Dune"}},
	}
	b2 = observation.Entity{Ref: observation.Ref{Kind: "book", ID: "b2"}, State: "lent"}
	a1 = observation.Entity{
		Ref:        observation.Ref{Kind: "author", ID: "a1"},
		Attributes: []observation.Attribute{{Name: "name", Value: "Frank Herbert"}},
	}
)

// fixture returns b1 and b2, books by a1, where b1 cites b2.
func fixture() ([]observation.Entity, []observation.Relation) {
	return []observation.Entity{b1, b2, a1}, []observation.Relation{
		{From: b1.Ref, Kind: "written_by", To: a1.Ref},
		{From: b2.Ref, Kind: "written_by", To: a1.Ref},
		{From: b1.Ref, Kind: "cites", To: b2.Ref},
	}
}

func TestNewSnapshot(t *testing.T) {
	entities, relations := fixture()

	s, err := observation.NewSnapshot(observedAt, entities, relations)

	require.NoError(t, err)
	require.Equal(t, observedAt, s.ObservedAt())
	require.Equal(t, []observation.Entity{b1, b2, a1}, s.Entities())
	require.Equal(t, relations, s.Relations())
}

func TestNewSnapshotWithoutEntities(t *testing.T) {
	s, err := observation.NewSnapshot(observedAt, nil, nil)

	require.NoError(t, err)
	require.Empty(t, s.Entities())
	require.Empty(t, s.Relations())
}

func TestSnapshotEntity(t *testing.T) {
	entities, relations := fixture()
	s, err := observation.NewSnapshot(observedAt, entities, relations)
	require.NoError(t, err)

	got, ok := s.Entity(b1.Ref)
	require.True(t, ok)
	require.Equal(t, b1, got)

	got, ok = s.Entity(observation.Ref{Kind: "book", ID: "zz"})
	require.False(t, ok)
	require.Equal(t, observation.Entity{}, got)
}

func TestSnapshotEntityDistinguishesKinds(t *testing.T) {
	book := observation.Entity{Ref: observation.Ref{Kind: "book", ID: "x"}, State: "b"}
	author := observation.Entity{Ref: observation.Ref{Kind: "author", ID: "x"}, State: "a"}

	s, err := observation.NewSnapshot(observedAt, []observation.Entity{book, author}, nil)
	require.NoError(t, err)

	got, ok := s.Entity(book.Ref)
	require.True(t, ok)
	require.Equal(t, book, got)
	got, ok = s.Entity(author.Ref)
	require.True(t, ok)
	require.Equal(t, author, got)
}

func TestZeroSnapshot(t *testing.T) {
	var s observation.Snapshot

	_, ok := s.Entity(b1.Ref)

	require.False(t, ok)
	require.Empty(t, s.Entities())
}

func TestNewSnapshotAllowsSelfRelation(t *testing.T) {
	_, err := observation.NewSnapshot(observedAt, []observation.Entity{b1},
		[]observation.Relation{{From: b1.Ref, Kind: "cites", To: b1.Ref}})

	require.NoError(t, err)
}

func TestNewSnapshotRejects(t *testing.T) {
	unknown := observation.Ref{Kind: "author", ID: "zz"}
	tests := map[string]struct {
		observedAt time.Time
		entities   []observation.Entity
		relations  []observation.Relation
		detail     string
	}{
		"zero time": {
			entities: []observation.Entity{b1},
			detail:   "observed time is zero",
		},
		"empty kind": {
			observedAt: observedAt,
			entities:   []observation.Entity{{Ref: observation.Ref{ID: "x"}}},
			detail:     "entity 0: empty kind",
		},
		"empty id": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, {Ref: observation.Ref{Kind: "book"}}},
			detail:     "entity 1: empty id",
		},
		"duplicate entity": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, a1, b1},
			detail:     "entity 2: duplicate book/b1",
		},
		"empty attribute name": {
			observedAt: observedAt,
			entities: []observation.Entity{{
				Ref:        b1.Ref,
				Attributes: []observation.Attribute{{Name: "title"}, {Value: "x"}},
			}},
			detail: "entity book/b1: attribute 1: empty name",
		},
		"duplicate attribute": {
			observedAt: observedAt,
			entities: []observation.Entity{{
				Ref:        b1.Ref,
				Attributes: []observation.Attribute{{Name: "title"}, {Name: "title"}},
			}},
			detail: `entity book/b1: duplicate attribute "title"`,
		},
		"empty relation kind": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, a1},
			relations:  []observation.Relation{{From: b1.Ref, To: a1.Ref}},
			detail:     "relation 0: empty kind",
		},
		"unknown source": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, a1},
			relations:  []observation.Relation{{From: unknown, Kind: "written_by", To: a1.Ref}},
			detail:     "relation 0: unknown source entity author/zz",
		},
		"unknown target": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, a1},
			relations:  []observation.Relation{{From: b1.Ref, Kind: "written_by", To: unknown}},
			detail:     "relation 0: unknown target entity author/zz",
		},
		"duplicate relation": {
			observedAt: observedAt,
			entities:   []observation.Entity{b1, a1},
			relations: []observation.Relation{
				{From: b1.Ref, Kind: "written_by", To: a1.Ref},
				{From: b1.Ref, Kind: "written_by", To: a1.Ref},
			},
			detail: "relation 1: duplicate book/b1 -written_by-> author/a1",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := observation.NewSnapshot(tc.observedAt, tc.entities, tc.relations)

			require.ErrorIs(t, err, observation.ErrInvalidSnapshot)
			require.ErrorContains(t, err, tc.detail)
		})
	}
}

func TestRefString(t *testing.T) {
	require.Equal(t, "book/b1", observation.Ref{Kind: "book", ID: "b1"}.String())
}
