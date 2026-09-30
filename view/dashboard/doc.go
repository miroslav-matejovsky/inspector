// Package dashboard renders an overview of a model.Model that explains what
// needs attention: the health of the entities by kind (chart.HealthByKind),
// the entities that are down and then degraded with the reason the system
// states, each linked to its entity page (entityPage?entity=<id>), and the
// issues of the model, the signals that could not be understood. At most 50
// entities are listed, followed by the number left out.
//
// The chart and the attention list sit side by side, in one column on
// screens up to 1100px wide; the issues span the full width below them.
//
// A page includes view.Styles, chart.Styles and Styles.
package dashboard
