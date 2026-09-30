---
title: "02 - View builders and state filter"
dependencies: []
effort: "S"
complexity: "low"
---

# 02 - View builders and state filter

## Objective

Every JSON view of `representation` is produced by a pure builder function that takes a snapshot and a `links` value holding the href base. The HTTP handlers only observe, call a builder and write JSON. The entity list gains the optional filter `state`. JSON output of every existing request is unchanged, so every existing test passes without edits. Steps 03 and 04 reuse the builders with the base `{prefix}/ui` for HTML.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/views.go` | New: `overviewOf`, `entityListOf`, `entityDetailOf`, `explanationViewOf`, `rootCausesOf`. |
| `representation/resources.go` | Type `links` replaces the href methods of `handler`; new `links.entitiesOf`. `explanationResourceOf` and `refResourceOf` become methods of `links`. |
| `representation/handler.go` | `handler` holds `api links`; handlers call the builders; entity list reads `state`. |
| `representation/handler_test.go` | New test `TestEntityListFilteredByState`. Existing tests unchanged. |
| `representation/doc.go` | Endpoint table: `?kind=` and `?state=`. |
| `harness/README.md` | Inspector endpoint table: `/inspector/entities` filters `kind` and `state`. |

## Implementation Tasks

1. Add `TestEntityListFilteredByState` to `handler_test.go`. Confirm it fails.
2. Create `links` in `resources.go` and move the href helpers onto it.
3. Create `views.go` with the builders; move the body of each handler into its builder.
4. Reduce each handler to observe, build, write.
5. Update `doc.go` and `harness/README.md`.
6. `go test ./representation/...`, then `task all`.

## Technical Details

### `links` (`resources.go`)

```go
// links builds the hrefs of one representation. base is the path under which
// the representation is served, for example "/inspector" for JSON views or
// "/inspector/ui" for HTML pages.
type links struct{ base string }

func (l links) entities() string                   // base + "/entities"
func (l links) entitiesOf(kind, state string) string
func (l links) entity(ref observation.Ref) string   // base + "/entities/" + PathEscape(kind) + "/" + PathEscape(id)
func (l links) explanation(ref observation.Ref) string // entity(ref) + "/explanation"
func (l links) ref(ref observation.Ref) refResource
func (l links) explanationResourceOf(e explanation.Explanation) explanationResource
```

`entitiesOf` adds only non-empty parameters, encoded with `url.Values.Encode` (keys sorted: `kind`, then `state`). With both empty it returns `entities()`. For a kind only it returns exactly what `kindHref` returns today (`?kind=` + `url.QueryEscape(kind)`), so overview hrefs do not change.

### Builders (`views.go`)

```go
// overviewOf lists the kinds in order of first appearance with their entity counts.
func overviewOf(s observation.Snapshot, l links) overviewResource

// entityListOf lists entities in snapshot order. A non-empty kind or state
// keeps only entities with that kind or state; both filters apply together.
func entityListOf(s observation.Snapshot, l links, kind, state string) entityListResource

// entityDetailOf returns one entity with attributes, history and related
// entities. It returns an error wrapping navigation.ErrUnknownEntity when ref
// is not an entity of s.
func entityDetailOf(s observation.Snapshot, l links, ref observation.Ref) (entityDetailResource, error)

// explanationViewOf returns the cause tree and root causes of ref. It returns
// an error wrapping explanation.ErrUnknownEntity when ref is not an entity of s.
func explanationViewOf(s observation.Snapshot, l links, ref observation.Ref) (explanationViewResource, error)

// rootCausesOf returns kind, id, state and href of each root cause of e, never nil.
func rootCausesOf(s observation.Snapshot, l links, e explanation.Explanation) []rootCauseResource
```

The code of each builder is the existing code of the matching handler, with `h.xxxHref` replaced by `l.xxx`.

### Handler (`handler.go`)

```go
type handler struct {
    prefix   string
    observer Observer
    api      links // base: prefix
}
```

`handleEntities` reads `kind` and `state` from the query and calls `entityListOf(snap, h.api, kind, state)`. The 404 and 500 mapping of `handleEntity` and `handleExplanation` stays in the handlers and uses `errors.Is` on the builder error.

### `doc.go`

Endpoint line: `GET {prefix}/entities   entities in snapshot order, optional filters ?kind= and ?state=`.

### Test (`handler_test.go`)

| Test | Assertion |
| --- | --- |
| `TestEntityListFilteredByState` | With `fixture(t)`: `?state=lent` lists only `book/b2`; `?kind=book&state=available` lists only `book/b1`; `?kind=author&state=lent` gives `"entities": []`; `?state=missing` gives `"entities": []`. Bodies compared with `require.JSONEq`. |

## Verification

```powershell
go test ./representation/...
task boundary
task all
```

## Acceptance Criteria

- `go test ./representation/...` exits 0, including `TestEntityListFilteredByState`.
- `git diff representation/handler_test.go` shows only the added test; no existing assertion changed.
- `representation/views.go` exists and contains the five builder functions named above.
- `Select-String -Path representation/*.go -Pattern 'func \(h \*handler\) \w*Href'` prints nothing.
- `task all` exits 0.

## Non-Goals

- HTML pages (step 03).
- Dashboard views (step 04).
- Filters other than `kind` and `state`, pagination, sorting.
