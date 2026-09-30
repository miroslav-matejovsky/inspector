// Package view is the Explain part of the Inspector toolkit: it presents what
// the toolkit knows about an inspected system in a form that humans read.
//
// Each kind of view is a sub-package:
//
//   - view/raw: the signals of a source.Source as they were collected.
//   - view/explorer: the entities of a model.Model by kind, and one entity with its links and backlinks.
//   - view/chart: SVG charts of a model.Model.
//   - view/dashboard: an overview of a model.Model: health chart, entities that need attention, issues.
//
// Views render HTML fragments with html/template, so every value taken from
// the inspected system is escaped. The root element of every view has the
// class "view" and a class of its own, "view-<name>"; the CSS of a view is
// scoped to its class and follows the light or dark color scheme of the
// browser.
//
// This package holds the CSS that every view shares, Styles: the palette,
// the classes num, note and error, and the health classes health-ok,
// health-degraded, health-down and health-unknown, which set --view-health
// for the classes health (colored text) and badge (filled label). A page
// includes Styles once, then the Styles of every view it shows.
package view
