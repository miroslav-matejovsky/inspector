package explanation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/explanation"
	"github.com/miroslav-matejovsky/inspector/observation"
)

var (
	observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	r1         = observation.Ref{Kind: "room", ID: "r1"}
	h1         = observation.Ref{Kind: "heater", ID: "h1"}
	f1         = observation.Ref{Kind: "fuse", ID: "f1"}
	l1         = observation.Ref{Kind: "lamp", ID: "l1"}
	h1History  = []observation.Transition{
		{To: "on", At: "day 1"},
		{From: "on", To: "off", At: "day 2", Reason: "no power"},
	}
)

// fixture returns a cold room heated by a heater that is off because its
// fuse blew. The lamp next to the room is unrelated to its state.
func fixture(t *testing.T) observation.Snapshot {
	t.Helper()
	return snapshot(t,
		[]observation.Entity{
			{Ref: r1, State: "cold", Reason: "no heat"},
			{Ref: h1, State: "off", Reason: "no power", History: h1History},
			{Ref: f1, State: "blown"},
			{Ref: l1, State: "dark"},
		},
		[]observation.Relation{
			{From: r1, Kind: "heated_by", To: h1, Cause: true},
			{From: h1, Kind: "powered_by", To: f1, Cause: true},
			{From: r1, Kind: "next_to", To: l1},
		})
}

func snapshot(t *testing.T, entities []observation.Entity, relations []observation.Relation) observation.Snapshot {
	t.Helper()
	s, err := observation.NewSnapshot(observedAt, entities, relations, nil)
	require.NoError(t, err)
	return s
}

// graph returns a snapshot of entities of kind "x" with the given IDs and
// cause relations of kind "c" between them, as pairs from, to.
func graph(t *testing.T, ids []string, causes ...[2]string) observation.Snapshot {
	t.Helper()
	var entities []observation.Entity
	for _, id := range ids {
		entities = append(entities, observation.Entity{Ref: x(id)})
	}
	var relations []observation.Relation
	for _, c := range causes {
		relations = append(relations, observation.Relation{From: x(c[0]), Kind: "c", To: x(c[1]), Cause: true})
	}
	return snapshot(t, entities, relations)
}

func x(id string) observation.Ref {
	return observation.Ref{Kind: "x", ID: id}
}

func TestExplainFollowsCauses(t *testing.T) {
	got, err := explanation.Explain(fixture(t), r1)

	require.NoError(t, err)
	require.Equal(t, explanation.Explanation{
		Ref: r1, State: "cold", Reason: "no heat",
		Causes: []explanation.Cause{{
			Relation: "heated_by",
			Explanation: explanation.Explanation{
				Ref: h1, State: "off", Reason: "no power", History: h1History,
				Causes: []explanation.Cause{{
					Relation:    "powered_by",
					Explanation: explanation.Explanation{Ref: f1, State: "blown"},
				}},
			},
		}},
	}, got)
}

func TestExplainIgnoresOtherRelations(t *testing.T) {
	s := fixture(t)

	lamp, err := explanation.Explain(s, l1)
	require.NoError(t, err)
	require.Empty(t, lamp.Causes)

	room, err := explanation.Explain(s, r1)
	require.NoError(t, err)
	require.Len(t, room.Causes, 1)
	require.Equal(t, h1, room.Causes[0].Explanation.Ref)
}

func TestRoots(t *testing.T) {
	s := fixture(t)

	room, err := explanation.Explain(s, r1)
	require.NoError(t, err)
	require.Equal(t, []observation.Ref{f1}, room.Roots())

	fuse, err := explanation.Explain(s, f1)
	require.NoError(t, err)
	require.Empty(t, fuse.Roots())
}

func TestExplainSharedCause(t *testing.T) {
	s := graph(t, []string{"a", "b1", "b2", "r"}, [2]string{"a", "b1"}, [2]string{"a", "b2"},
		[2]string{"b1", "r"}, [2]string{"b2", "r"})

	got, err := explanation.Explain(s, x("a"))

	require.NoError(t, err)
	require.Len(t, got.Causes, 2)
	first := got.Causes[0].Explanation.Causes
	require.Len(t, first, 1)
	require.False(t, first[0].Repeated)
	second := got.Causes[1].Explanation.Causes
	require.Equal(t, []explanation.Cause{
		{Relation: "c", Explanation: explanation.Explanation{Ref: x("r")}, Repeated: true},
	}, second)
	require.Equal(t, []observation.Ref{x("r")}, got.Roots())
}

func TestExplainCycle(t *testing.T) {
	s := graph(t, []string{"a", "b"}, [2]string{"a", "b"}, [2]string{"b", "a"})

	got, err := explanation.Explain(s, x("a"))

	require.NoError(t, err)
	require.Equal(t, []explanation.Cause{{
		Relation: "c",
		Explanation: explanation.Explanation{Ref: x("b"), Causes: []explanation.Cause{
			{Relation: "c", Explanation: explanation.Explanation{Ref: x("a")}, Repeated: true},
		}},
	}}, got.Causes)
	require.Empty(t, got.Roots())
}

func TestExplainSelfCause(t *testing.T) {
	s := graph(t, []string{"a"}, [2]string{"a", "a"})

	got, err := explanation.Explain(s, x("a"))

	require.NoError(t, err)
	require.Equal(t, []explanation.Cause{
		{Relation: "c", Explanation: explanation.Explanation{Ref: x("a")}, Repeated: true},
	}, got.Causes)
}

func TestExplainUnknownEntity(t *testing.T) {
	_, err := explanation.Explain(fixture(t), observation.Ref{Kind: "room", ID: "zz"})

	require.ErrorIs(t, err, explanation.ErrUnknownEntity)
	require.ErrorContains(t, err, "room/zz")
}
