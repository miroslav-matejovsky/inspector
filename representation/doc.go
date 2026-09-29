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
//
// # Freshness
//
// Every request calls Observer.Observe once. There is no caching, so every
// view shows the current state of the source.
//
// # JSON
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
