// Package navigation is the Navigation domain of Inspector: it traverses the
// relations of an observed snapshot.
//
// Core questions:
//
//	What is related?
//	How is this connected?
//	Where did this come from?
//
// Links returns every relation that touches one entity as a Link. An
// Outgoing link starts at the entity (the entity is Relation.From), an
// Incoming link ends at it (the entity is Relation.To). Target is always the
// entity on the other end. Outgoing links come first, then incoming links,
// each in the order of the snapshot relations. A relation from an entity to
// itself yields one link of each direction. Asking for an entity that is not
// in the snapshot is an error wrapping ErrUnknownEntity.
//
// Links scans all relations of the snapshot; there is no index.
package navigation
