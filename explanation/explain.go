package explanation

import (
	"errors"
	"fmt"

	"github.com/miroslav-matejovsky/inspector/observation"
)

// ErrUnknownEntity marks a Ref that is not an entity of the snapshot.
var ErrUnknownEntity = errors.New("unknown entity")

// Explanation tells why one entity is in its current state.
type Explanation struct {
	Ref     observation.Ref
	State   string
	Reason  string                   // as reported by the source
	History []observation.Transition // as reported by the source, oldest first
	Causes  []Cause                  // one per cause relation from Ref, in snapshot relation order
}

// Cause is one entity whose state is a cause of the explained state.
type Cause struct {
	Relation    string      // kind of the cause relation
	Explanation Explanation // why the cause entity is in its state
	Repeated    bool        // the entity is already expanded earlier in this explanation; Explanation.Causes is empty
}

// Explain returns why the entity ref is in its current state. It follows
// relations with Cause set, outward from ref, depth first in snapshot
// relation order. Every entity is expanded once; later occurrences are marked
// Repeated. It returns an error wrapping ErrUnknownEntity when ref is not an
// entity of s.
func Explain(s observation.Snapshot, ref observation.Ref) (Explanation, error) {
	if _, ok := s.Entity(ref); !ok {
		return Explanation{}, fmt.Errorf("explanation: %w: %s", ErrUnknownEntity, ref)
	}
	expanded := map[observation.Ref]bool{}
	var expand func(ref observation.Ref) Explanation
	expand = func(ref observation.Ref) Explanation {
		e := facts(s, ref)
		// Marked before the causes, so that cycles and self causes end in a
		// Repeated cause instead of recursing forever.
		expanded[ref] = true
		for _, r := range s.Relations() {
			if r.From != ref || !r.Cause {
				continue
			}
			if expanded[r.To] {
				e.Causes = append(e.Causes, Cause{Relation: r.Kind, Explanation: facts(s, r.To), Repeated: true})
				continue
			}
			e.Causes = append(e.Causes, Cause{Relation: r.Kind, Explanation: expand(r.To)})
		}
		return e
	}
	return expand(ref), nil
}

// facts returns the explanation of ref without causes. Relations always point
// to entities of the snapshot, so ref exists.
func facts(s observation.Snapshot, ref observation.Ref) Explanation {
	entity, _ := s.Entity(ref)
	return Explanation{Ref: ref, State: entity.State, Reason: entity.Reason, History: entity.History}
}

// Roots returns the root causes: entities reached through causes that have
// no causes of their own, depth first, each once. Repeated occurrences are
// skipped. An explanation without causes has no roots.
func (e Explanation) Roots() []observation.Ref {
	var roots []observation.Ref
	seen := map[observation.Ref]bool{}
	var walk func(causes []Cause)
	walk = func(causes []Cause) {
		for _, c := range causes {
			switch {
			case c.Repeated:
			case len(c.Explanation.Causes) == 0:
				if !seen[c.Explanation.Ref] {
					seen[c.Explanation.Ref] = true
					roots = append(roots, c.Explanation.Ref)
				}
			default:
				walk(c.Explanation.Causes)
			}
		}
	}
	walk(e.Causes)
	return roots
}
