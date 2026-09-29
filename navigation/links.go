package navigation

import (
	"errors"
	"fmt"

	"github.com/miroslav-matejovsky/inspector/observation"
)

// ErrUnknownEntity marks a Ref that is not an entity of the snapshot.
var ErrUnknownEntity = errors.New("unknown entity")

// Direction tells on which end of a relation the starting entity is.
type Direction string

// Directions of a Link.
const (
	Outgoing Direction = "outgoing" // the starting entity is Relation.From
	Incoming Direction = "incoming" // the starting entity is Relation.To
)

// Link is one step from an entity to a related entity.
type Link struct {
	Relation  string // observation.Relation.Kind
	Direction Direction
	Target    observation.Ref // the entity on the other end
	Cause     bool            // copied from the relation, in both directions
}

// Links returns every link of the entity ref in s: first all outgoing links,
// then all incoming links, each group in the order of s.Relations(). A
// relation from an entity to itself yields one outgoing and one incoming link.
// An entity without relations has no links. Links returns an error wrapping
// ErrUnknownEntity when ref is not an entity of s.
func Links(s observation.Snapshot, ref observation.Ref) ([]Link, error) {
	if _, ok := s.Entity(ref); !ok {
		return nil, fmt.Errorf("navigation: %w: %s", ErrUnknownEntity, ref)
	}
	links := []Link{}
	for _, r := range s.Relations() {
		if r.From == ref {
			links = append(links, Link{Relation: r.Kind, Direction: Outgoing, Target: r.To, Cause: r.Cause})
		}
	}
	for _, r := range s.Relations() {
		if r.To == ref {
			links = append(links, Link{Relation: r.Kind, Direction: Incoming, Target: r.From, Cause: r.Cause})
		}
	}
	return links, nil
}
