package model_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/source"
)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

// summary returns the summary of the target name with the latest signal sig.
func summary(name string, sig *source.Signal) source.TargetSummary {
	return source.TargetSummary{
		Target: source.Target{Name: name, URL: "http://example.test/" + name},
		Latest: sig,
	}
}

// signal returns a read signal of the target name.
func signal(name string) *source.Signal {
	return &source.Signal{
		Target:      name,
		URL:         "http://example.test/" + name,
		ObservedAt:  t0,
		StatusCode:  200,
		ContentType: "application/json",
		Body:        []byte(`{}`),
	}
}

// byName selects interpreters by Target.Name.
func byName(m map[string]model.Interpreter) func(source.Target) model.Interpreter {
	return func(t source.Target) model.Interpreter { return m[t.Name] }
}

// returns is an interpreter that returns entities with the given IDs.
func returns(ids ...string) model.Interpreter {
	return func(source.Signal) ([]model.Entity, error) {
		var out []model.Entity
		for _, id := range ids {
			out = append(out, model.Entity{ID: id, Kind: "k"})
		}
		return out, nil
	}
}

func ids(m *model.Model) []string {
	var out []string
	for _, e := range m.Entities() {
		out = append(out, e.ID)
	}
	return out
}

func TestBuildReadsEntitiesInTargetOrder(t *testing.T) {
	m := model.Build(
		[]source.TargetSummary{summary("a", signal("a")), summary("b", signal("b"))},
		byName(map[string]model.Interpreter{"a": returns("x1", "x2"), "b": returns("y")}),
	)
	require.Equal(t, []string{"x1", "x2", "y"}, ids(m))
	require.Empty(t, m.Issues())
}

func TestBuildSetsEvidenceFromSignal(t *testing.T) {
	interpret := func(source.Signal) ([]model.Entity, error) {
		return []model.Entity{{ID: "x", Kind: "k", Evidence: model.Evidence{Target: "fake"}}}, nil
	}
	m := model.Build(
		[]source.TargetSummary{summary("a", signal("a"))},
		byName(map[string]model.Interpreter{"a": interpret}),
	)
	e, ok := m.Entity("x")
	require.True(t, ok)
	require.Equal(t, model.Evidence{Target: "a", URL: "http://example.test/a", ObservedAt: t0}, e.Evidence)
}

func TestBuildPassesSignalToInterpreter(t *testing.T) {
	sig := signal("a")
	sig.StatusCode = 503
	sig.ContentType = "text/plain"
	sig.Body = []byte("down")
	var got source.Signal
	interpret := func(s source.Signal) ([]model.Entity, error) {
		got = s
		return nil, nil
	}
	model.Build(
		[]source.TargetSummary{summary("a", sig)},
		byName(map[string]model.Interpreter{"a": interpret}),
	)
	require.Equal(t, *sig, got)
}

func TestBuildSkipsTargetsWithoutInterpreter(t *testing.T) {
	m := model.Build(
		[]source.TargetSummary{summary("a", signal("a")), summary("b", nil)},
		byName(nil),
	)
	require.Empty(t, m.Entities())
	require.Empty(t, m.Issues())
}

func TestBuildReportsTargetWithoutSignal(t *testing.T) {
	m := model.Build(
		[]source.TargetSummary{summary("a", nil)},
		byName(map[string]model.Interpreter{"a": returns("x")}),
	)
	require.Equal(t, []model.Issue{{Target: "a", Error: "no signal collected yet"}}, m.Issues())
	require.Empty(t, m.Entities())
}

func TestBuildReportsFailedRead(t *testing.T) {
	sig := signal("a")
	sig.Body = nil
	sig.Error = "connection refused"
	interpret := func(source.Signal) ([]model.Entity, error) {
		t.Fatal("interpreter called for a failed read")
		return nil, nil
	}
	m := model.Build(
		[]source.TargetSummary{summary("a", sig)},
		byName(map[string]model.Interpreter{"a": interpret}),
	)
	require.Equal(t, []model.Issue{{Target: "a", Error: "read failed: connection refused"}}, m.Issues())
}

func TestBuildReportsInterpreterError(t *testing.T) {
	failing := func(source.Signal) ([]model.Entity, error) {
		return []model.Entity{{ID: "x", Kind: "k"}}, errors.New("boom")
	}
	m := model.Build(
		[]source.TargetSummary{summary("a", signal("a")), summary("b", signal("b"))},
		byName(map[string]model.Interpreter{"a": failing, "b": returns("y")}),
	)
	require.Equal(t, []model.Issue{{Target: "a", Error: "interpret: boom"}}, m.Issues())
	_, ok := m.Entity("x")
	require.False(t, ok)
	_, ok = m.Entity("y")
	require.True(t, ok)
}
