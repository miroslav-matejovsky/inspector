// Package representation is the Representation domain of Inspector: it
// presents observed snapshots in forms that humans and tools can consume.
//
// Core questions:
//
//	How should this be presented?
//	How can understanding be communicated?
//
// # Endpoints
//
// NewHandler serves JSON views under a prefix chosen by the host:
//
//	GET {prefix}/                      overview: observation time, kinds with entity counts
//	GET {prefix}/entities              entities in snapshot order, optional filter ?kind=
//	GET {prefix}/entities/{kind}/{id}  one entity with attributes and related entities
//	GET {prefix}/entities/{kind}/{id}/explanation
//	                                   why the entity is in its state: cause tree and root causes
//
// # Freshness
//
// Every request calls Observer.Observe once. There is no caching, so every
// view shows the current state of the source.
//
// # JSON
//
// Entity views have "reason" (omitted when empty) and "history" (never null,
// oldest first, each entry with "from", "to", "at" and "reason" as reported
// by the source).
//
// Entity views link their explanation with "explanation_href". The explanation
// view has "explanation": the entity with kind, id, state, reason, href,
// history and "causes", where each cause adds "relation" and, when the entity
// is already expanded earlier in the tree, "repeated": true; and
// "root_causes": kind, id, state and href of each root cause, never null. The
// tree comes from package explanation.
//
// Related items of cause relations have "cause": true; other related items
// have no "cause" key.
//
// Every view has "gaps": the parts of the source that could not be observed,
// each with "source" and "error". A view with gaps is incomplete.
//
// Names are snake_case. Every addressable item carries an href, so a client
// navigates from the overview to a kind, an entity and its related entities
// without building URLs. Kind and ID are path-escaped in hrefs. A state is
// omitted when empty. Lists (kinds, entities, attributes, related) are never
// null. Related entities follow navigation.Links: outgoing first, then
// incoming. Errors have the form
//
//	{"error":{"code":"<code>","message":"<text>"}}
//
// with these codes:
//
//	source_unavailable  503  the observer failed; message is its error
//	entity_not_found    404  no such entity in the snapshot
//	internal_error      500  a view cannot be encoded
//
// Methods other than GET are answered 405 by http.ServeMux. The handler does
// not log; errors are visible in the response.
//
// # Mounting
//
// The host mounts the handler at prefix+"/" and does not strip the prefix.
package representation
