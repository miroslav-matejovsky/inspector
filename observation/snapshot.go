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

// Entity is one thing that exists in the inspected system.
type Entity struct {
	Ref        Ref
	State      string      // current state as named by the source; empty when the source reports none
	Attributes []Attribute // in source order; nil when there are none
}

// Relation is a directed, named connection between two entities.
type Relation struct {
	From Ref
	Kind string // name of the connection as chosen by the source, not empty
	To   Ref
}

// Snapshot is the state of one inspected system at one point in time.
// The zero value is an empty snapshot with a zero observation time.
type Snapshot struct {
	observedAt time.Time
	entities   []Entity
	relations  []Relation
	index      map[Ref]int // position of each entity in entities
}

// NewSnapshot validates its input and returns a snapshot. It keeps the given
// slices; the caller must not modify them afterwards. The first violated
// invariant is returned as an error wrapping ErrInvalidSnapshot.
func NewSnapshot(observedAt time.Time, entities []Entity, relations []Relation) (Snapshot, error) {
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
	}

	seen := make(map[Relation]bool, len(relations))
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
		if seen[r] {
			return Snapshot{}, invalid("relation %d: duplicate %s -%s-> %s", k, r.From, r.Kind, r.To)
		}
		seen[r] = true
	}

	return Snapshot{observedAt: observedAt, entities: entities, relations: relations, index: index}, nil
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

// Entity returns the entity with ref and whether it exists.
func (s Snapshot) Entity(ref Ref) (Entity, bool) {
	i, ok := s.index[ref]
	if !ok {
		return Entity{}, false
	}
	return s.entities[i], true
}
