// Package explanation is the Explanation domain of Inspector: it tells why an
// observed entity is in its current state.
//
// Core questions:
//
//	Why does this exist?
//	Why is it in this state?
//	Why did this happen?
//
// # Causes
//
// The library never decides what causes what. Explain follows the relations
// whose Cause flag the source set, and shows the reasons and histories the
// source reported. States, reasons and relation kinds stay opaque strings.
//
// # Tree and roots
//
// An Explanation holds the state, reason and history of one entity and one
// Cause per cause relation leaving it, each with the explanation of the
// causing entity. The tree is built depth first in snapshot relation order.
// Every entity is expanded once; a later occurrence, including a cycle back
// to an entity on the path, is marked Repeated and carries no causes. Roots
// returns the root causes: the entities at the ends of the tree that have no
// causes of their own.
//
// # Time
//
// An explanation shows the current state of every causing entity. A cause can
// be historical: an entity may have reached its state because of a state its
// cause no longer has. The history of the explained entity shows when and
// why its state changed.
package explanation
