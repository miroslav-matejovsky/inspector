// Package chart draws charts of a model.Model as inline SVG, rendered with
// html/template so that every label from the inspected system is escaped.
// Charts need no script.
//
// HealthByKind draws one bar per kind of entity, split by health (ok,
// degraded, down, unknown) in proportion to the number of entities; every
// bar has the same length, so kinds with few and with many entities can be
// compared by share.
//
// A page includes view.Styles and Styles; the colors are the health colors
// of view.Styles.
package chart
