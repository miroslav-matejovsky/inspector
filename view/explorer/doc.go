// Package explorer renders a model.Model for navigation (Navigate -> Model,
// Explain -> View).
//
// Index lists the entities of every kind, in the order of the model; Entity
// shows one entity with its state, properties, the evidence it was read
// from, its links and its backlinks. Every related entity is a link to its
// entity page, entityPage?entity=<id>, which the page that embeds the view
// serves with Entity; a related ID that is not in the model is shown as
// "not observed". Index shows at most 100 entities per kind and Entity at
// most 100 backlinks, each followed by the number left out.
//
// A page includes view.Styles and Styles.
package explorer
