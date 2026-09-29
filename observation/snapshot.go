package observation

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidSnapshot marks input that violates an invariant of NewSnapshot.
var ErrInvalidSnapshot = errors.New("invalid snapshot")

// Ref identifies one entity of an inspected system. Kind groups entities of
// the same type; ID is unique within its Kind. Both are chosen by the source
// and are opaque to Inspector.
type Ref struct {
	Kind string
	ID   string
}

// String returns "kind/id". It is meant for messages, not for parsing.
func (r Ref) String() string {
	return r.Kind + "/" + r.ID
}

// Attribute is one named fact about an entity, as reported by the source.
type Attribute struct {
	Name  string // not empty, unique within the entity
	Value string // may be empty
}

// Transition is one recorded change of an entity's state, as reported by the
// source.
type Transition struct {
	From   string // state before; empty for the first recorded state
	To     string // state after, not empty
	At     string // when, in the source's own notation, for example "day 3"; may be empty
	Reason string // why, as named by the source; may be empty
}

// Entity is one thing that exists in the inspected system.
type Entity struct {
	Ref        Ref
	State      string       // current state as named by the source; empty when the source reports none
	Reason     string       // why the entity is in its current state, as reported by the source; empty when unknown
	History    []Transition // recorded state changes, oldest first; nil when the source reports none
	Attributes []Attribute  // in source order; nil when there are none
}

// Relation is a directed, named connection between two entities.
type Relation struct {
	From  Ref
	Kind  string // name of the connection as chosen by the source, not empty
	To    Ref
	Cause bool // the state of To is a cause of the current state of From, as the source reports it
}

// Gap is a part of the source that could not be observed. A snapshot with
// gaps is incomplete: the entities and relations of that part are missing.
type Gap struct {
	Source string // the part of the source, as named by the consumer, not empty
	Error  string // why it could not be observed, not empty
}

// Snapshot is the state of one inspected system at one point in time.
// The zero value is an empty snapshot with a zero observation time.
type Snapshot struct {
	observedAt time.Time
	entities   []Entity
	relations  []Relation
	gaps       []Gap
	index      map[Ref]int // position of each entity in entities
}

// NewSnapshot validates its input and returns a snapshot. It keeps the given
// slices; the caller must not modify them afterwards. The first violated
// invariant is returned as an error wrapping ErrInvalidSnapshot.
func NewSnapshot(observedAt time.Time, entities []Entity, relations []Relation, gaps []Gap) (Snapshot, error) {
	if observedAt.IsZero() {
		return Snapshot{}, invalid("observed time is zero")
	}

	index := make(map[Ref]int, len(entities))
	for i, e := range entities {
		switch {
		case e.Ref.Kind == "":
			return Snapshot{}, invalid("entity %d: empty kind", i)
		case e.Ref.ID == "":
			return Snapshot{}, invalid("entity %d: empty id", i)
		}
		if _, dup := index[e.Ref]; dup {
			return Snapshot{}, invalid("entity %d: duplicate %s", i, e.Ref)
		}
		index[e.Ref] = i

		names := make(map[string]bool, len(e.Attributes))
		for j, a := range e.Attributes {
			if a.Name == "" {
				return Snapshot{}, invalid("entity %s: attribute %d: empty name", e.Ref, j)
			}
			if names[a.Name] {
				return Snapshot{}, invalid("entity %s: duplicate attribute %q", e.Ref, a.Name)
			}
			names[a.Name] = true
		}
		for j, tr := range e.History {
			if tr.To == "" {
				return Snapshot{}, invalid("entity %s: transition %d: empty target state", e.Ref, j)
			}
		}
	}

	// A relation is identified by From, Kind and To; Cause does not make it distinct.
	type relationKey struct {
		from, to Ref
		kind     string
	}
	seen := make(map[relationKey]bool, len(relations))
	for k, r := range relations {
		if r.Kind == "" {
			return Snapshot{}, invalid("relation %d: empty kind", k)
		}
		if _, ok := index[r.From]; !ok {
			return Snapshot{}, invalid("relation %d: unknown source entity %s", k, r.From)
		}
		if _, ok := index[r.To]; !ok {
			return Snapshot{}, invalid("relation %d: unknown target entity %s", k, r.To)
		}
		key := relationKey{from: r.From, to: r.To, kind: r.Kind}
		if seen[key] {
			return Snapshot{}, invalid("relation %d: duplicate %s -%s-> %s", k, r.From, r.Kind, r.To)
		}
		seen[key] = true
	}

	for i, g := range gaps {
		switch {
		case g.Source == "":
			return Snapshot{}, invalid("gap %d: empty source", i)
		case g.Error == "":
			return Snapshot{}, invalid("gap %d: empty error", i)
		}
	}

	return Snapshot{observedAt: observedAt, entities: entities, relations: relations, gaps: gaps, index: index}, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("observation: %w: %s", ErrInvalidSnapshot, fmt.Sprintf(format, args...))
}

// ObservedAt returns the time at which the source was observed.
func (s Snapshot) ObservedAt() time.Time {
	return s.observedAt
}

// Entities returns all entities in source order. The caller must not modify
// the returned slice.
func (s Snapshot) Entities() []Entity {
	return s.entities
}

// Relations returns all relations in source order. The caller must not modify
// the returned slice.
func (s Snapshot) Relations() []Relation {
	return s.relations
}

// Gaps returns the parts of the source that could not be observed, in the
// order the consumer recorded them. The caller must not modify the slice.
func (s Snapshot) Gaps() []Gap {
	return s.gaps
}

// Entity returns the entity with ref and whether it exists.
func (s Snapshot) Entity(ref Ref) (Entity, bool) {
	i, ok := s.index[ref]
	if !ok {
		return Entity{}, false
	}
	return s.entities[i], true
}
