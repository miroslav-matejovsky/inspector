// Package observation is the Observation domain of Inspector: it exposes the
// current state of an inspected system as facts, without interpretation.
//
// Core questions:
//
//	What exists?
//	What is its current state?
//	What does the system currently know?
//
// # Model
//
// A Snapshot is the state of one inspected system at one point in time. It
// holds entities and relations:
//
//   - Entity: one thing that exists. It is identified by a Ref (Kind and ID),
//     has an optional State, a Reason for that state, a History of
//     Transitions that led to it, and ordered Attributes (name and value).
//   - Relation: a directed, named connection From one entity To another.
//
// Reason and History are reported by the source, as it knows them. A
// Transition's At is written in the source's own notation and is never
// interpreted; the History is ordered oldest first. The library does not
// check that the last transition matches the State: the source is
// responsible for its own consistency.
//
// A relation with Cause set is a cause relation: the state of To is a cause
// of the current state of From. The library never decides what causes what;
// the source states it and the consumer maps it. A relation is identified by
// From, Kind and To; two relations that differ only in Cause are duplicates.
//
// # Vocabulary
//
// Kinds, IDs, states, attribute names and relation kinds are strings chosen
// by the source. Inspector treats them as opaque and never hardcodes the
// vocabulary of an inspected system. A consumer maps its own system into
// this model.
//
// # Invariants
//
// NewSnapshot is the only way to build a valid Snapshot. It requires a
// non-zero observation time; a non-empty kind and ID for every entity and a
// Ref unique across entities; non-empty attribute names unique within their
// entity; a non-empty kind for every relation, both ends of every relation
// among the entities, and no duplicate relation. Empty states, empty
// attribute values, relations from an entity to itself and snapshots without
// entities are allowed. Violations are errors wrapping ErrInvalidSnapshot.
//
// # Gaps
//
// A snapshot can be partial. Each Gap names a part of the source that could
// not be observed and why. The entities and relations of that part are
// missing from the snapshot. A gap needs a non-empty source and error.
//
// # Ownership
//
// NewSnapshot keeps the given slices and the accessors return the stored
// slices. Neither the caller nor any reader may modify them.
package observation
