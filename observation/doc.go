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
//     has an optional State and ordered Attributes (name and value).
//   - Relation: a directed, named connection From one entity To another.
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
// # Ownership
//
// NewSnapshot keeps the given slices and the accessors return the stored
// slices. Neither the caller nor any reader may modify them.
package observation
