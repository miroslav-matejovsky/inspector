package model

import (
	"fmt"
	"slices"
	"time"
)

// Health is how well an entity does, as its signal states it. The set is
// closed: views color by it.
type Health string

// Health values.
const (
	HealthUnknown  Health = "unknown" // the signal states no health
	HealthOK       Health = "ok"
	HealthDegraded Health = "degraded"
	HealthDown     Health = "down"
)

// State is the condition of an entity in the words of the inspected system.
type State struct {
	Value  string // for example "outage" or "pending"; empty when the signal states none
	Health Health // empty is read as HealthUnknown by New
	Reason string // why the entity is in this state; empty when the signal states none
}

// Property is one named fact about an entity, as its signal states it.
type Property struct {
	Name  string
	Value string
}

// Link is a relation from the entity that holds it to the entity with the ID To.
type Link struct {
	Type string // what the relation means, read from the holder to To, for example "waits on"
	To   string // ID of the related entity; it may be missing from the model
}

// Backlink is a Link seen from the entity it points to.
type Backlink struct {
	From string // ID of the entity that holds the Link
	Type string // Link.Type
}

// Evidence is the signal an entity was read from.
type Evidence struct {
	Target     string    // name of the target
	URL        string    // URL of the target
	ObservedAt time.Time // start of the read, UTC
}

// Entity is one thing of the inspected system.
type Entity struct {
	ID         string     // unique in a Model and the same in every build
	Kind       string     // what the entity is, for example "order"; interpreters define kinds
	Name       string     // short name for humans; New sets an empty Name to ID
	State      State      // condition of the entity
	Properties []Property // in the order the interpreter gives them
	Links      []Link     // in the order the interpreter gives them
	Evidence   Evidence   // set by Build
}

// Issue is a signal, or an entity read from it, that could not be modeled.
type Issue struct {
	Target string // name of the target of the signal
	Error  string
}

// Model is a read-only snapshot of the inspected system. The slices returned
// by its methods must not be modified.
type Model struct {
	entities  []Entity              // in the order of New
	byID      map[string]int        // Entity.ID -> index in entities
	backlinks map[string][]Backlink // Link.To -> links to it, in entity and link order
	issues    []Issue               // given issues first, then those found by New
}

// New returns the model of entities, in their order. An entity that is not
// valid, see validate, or whose ID an earlier entity has, is left out and
// reported as an Issue of its Evidence.Target, after the given issues. New
// sets an empty State.Health to HealthUnknown and an empty Name to ID. Links
// to IDs that no entity has are kept.
func New(entities []Entity, issues []Issue) *Model {
	m := &Model{
		byID:      map[string]int{},
		backlinks: map[string][]Backlink{},
		issues:    slices.Clone(issues),
	}
	for _, e := range entities {
		if err := validate(e); err != nil {
			m.issues = append(m.issues, Issue{Target: e.Evidence.Target, Error: err.Error()})
			continue
		}
		if i, ok := m.byID[e.ID]; ok {
			m.issues = append(m.issues, Issue{
				Target: e.Evidence.Target,
				Error:  fmt.Sprintf("entity %q: already read from target %q", e.ID, m.entities[i].Evidence.Target),
			})
			continue
		}
		if e.State.Health == "" {
			e.State.Health = HealthUnknown
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		m.byID[e.ID] = len(m.entities)
		m.entities = append(m.entities, e)
	}
	for _, e := range m.entities {
		for _, l := range e.Links {
			m.backlinks[l.To] = append(m.backlinks[l.To], Backlink{From: e.ID, Type: l.Type})
		}
	}
	return m
}

// validate returns why e cannot be part of a model, or nil.
func validate(e Entity) error {
	switch {
	case e.ID == "":
		return fmt.Errorf("entity of kind %q named %q: empty id", e.Kind, e.Name)
	case e.Kind == "":
		return fmt.Errorf("entity %q: empty kind", e.ID)
	}
	switch e.State.Health {
	case "", HealthUnknown, HealthOK, HealthDegraded, HealthDown:
	default:
		return fmt.Errorf("entity %q: unknown health %q", e.ID, e.State.Health)
	}
	for i, l := range e.Links {
		if l.Type == "" || l.To == "" {
			return fmt.Errorf("entity %q: link %d: empty type or target", e.ID, i)
		}
	}
	return nil
}

// Entities returns every entity of the model, in the order of New.
func (m *Model) Entities() []Entity { return m.entities }

// Entity returns the entity with the ID id; ok is false when there is none.
func (m *Model) Entity(id string) (e Entity, ok bool) {
	i, ok := m.byID[id]
	if !ok {
		return Entity{}, false
	}
	return m.entities[i], true
}

// Backlinks returns the links to id, in entity and link order. id does not
// have to be an entity of the model.
func (m *Model) Backlinks(id string) []Backlink { return m.backlinks[id] }

// Issues returns what could not be modeled: the issues given to New first,
// then the entities that New left out, in their order.
func (m *Model) Issues() []Issue { return m.issues }
