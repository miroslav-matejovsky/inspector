package navigation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/navigation"
	"github.com/miroslav-matejovsky/inspector/observation"
)

var (
	observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	b1         = observation.Ref{Kind: "book", ID: "b1"}
	b2         = observation.Ref{Kind: "book", ID: "b2"}
	a1         = observation.Ref{Kind: "author", ID: "a1"}
)

// fixture returns b1 and b2, books by a1, where b1 cites b2.
func fixture(t *testing.T) observation.Snapshot {
	t.Helper()
	s, err := observation.NewSnapshot(observedAt,
		[]observation.Entity{{Ref: b1}, {Ref: b2}, {Ref: a1}},
		[]observation.Relation{
			{From: b1, Kind: "written_by", To: a1},
			{From: b2, Kind: "written_by", To: a1},
			{From: b1, Kind: "cites", To: b2},
		})
	require.NoError(t, err)
	return s
}

func TestLinksOutgoingThenIncoming(t *testing.T) {
	s := fixture(t)
	tests := map[string]struct {
		from observation.Ref
		want []navigation.Link
	}{
		"b1": {b1, []navigation.Link{
			{Relation: "written_by", Direction: navigation.Outgoing, Target: a1},
			{Relation: "cites", Direction: navigation.Outgoing, Target: b2},
		}},
		"b2": {b2, []navigation.Link{
			{Relation: "written_by", Direction: navigation.Outgoing, Target: a1},
			{Relation: "cites", Direction: navigation.Incoming, Target: b1},
		}},
		"a1": {a1, []navigation.Link{
			{Relation: "written_by", Direction: navigation.Incoming, Target: b1},
			{Relation: "written_by", Direction: navigation.Incoming, Target: b2},
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := navigation.Links(s, tc.from)

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLinksWithoutRelations(t *testing.T) {
	lonely := observation.Ref{Kind: "book", ID: "lonely"}
	s, err := observation.NewSnapshot(observedAt, []observation.Entity{{Ref: lonely}}, nil)
	require.NoError(t, err)

	got, err := navigation.Links(s, lonely)

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestLinksSelfRelation(t *testing.T) {
	x := observation.Ref{Kind: "book", ID: "x"}
	s, err := observation.NewSnapshot(observedAt, []observation.Entity{{Ref: x}},
		[]observation.Relation{{From: x, Kind: "cites", To: x}})
	require.NoError(t, err)

	got, err := navigation.Links(s, x)

	require.NoError(t, err)
	require.Equal(t, []navigation.Link{
		{Relation: "cites", Direction: navigation.Outgoing, Target: x},
		{Relation: "cites", Direction: navigation.Incoming, Target: x},
	}, got)
}

func TestLinksUnknownEntity(t *testing.T) {
	_, err := navigation.Links(fixture(t), observation.Ref{Kind: "book", ID: "zz"})

	require.ErrorIs(t, err, navigation.ErrUnknownEntity)
	require.ErrorContains(t, err, "book/zz")
}
