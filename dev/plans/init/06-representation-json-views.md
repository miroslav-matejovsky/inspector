---
title: "06 - Representation JSON views"
dependencies: ["02-observation-model", "03-navigation-links"]
effort: "M"
complexity: "medium"
---

# 06 - Representation JSON views

## Objective

Package `representation` serves JSON views of the current snapshot over HTTP: an overview of what exists, a list of entities and one entity with its attributes and related entities. Every view observes the source again through a consumer-provided `Observer`, so it always shows current state. Every addressable item carries an `href`, so a client navigates without building URLs. The package knows no inspected system; the consumer chooses the mount prefix.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/doc.go` | New: package documentation (replaces `README.md`). |
| `representation/README.md` | Deleted. Its purpose and core questions move to `doc.go`. |
| `representation/handler.go` | New: `Observer`, `NewHandler`, the three view handlers, error writing. |
| `representation/resources.go` | New: unexported JSON resource types and href builders. |
| `representation/handler_test.go` | New tests, package `representation_test`. |
| `.go-arch-lint.yml` | Component `representation: { in: representation }`; `deps`: `representation: { mayDependOn: [observation, navigation] }`. |
| `README.md` (root) | Row `representation` in the `## Library` package table. |

## Implementation Tasks

1. Write `representation/handler_test.go` with every test listed below. Confirm it fails to compile.
2. Create `resources.go`, then `handler.go`.
3. Create `representation/doc.go`; delete `representation/README.md`.
4. Add the component and the rule to `.go-arch-lint.yml`.
5. Add the table row to root `README.md`.
6. Run `go test ./representation/...` until it passes, then `task all`.

## Technical Details

### API (`handler.go`)

```go
// Observer provides the current snapshot of an inspected system. The handler
// calls it once per request, with the request context.
type Observer interface {
    Observe(ctx context.Context) (observation.Snapshot, error)
}

// NewHandler returns the JSON views of the snapshots of observer, served under
// prefix, for example "/inspector". prefix is one or more path segments of
// letters, digits, ".", "_", "~" or "-", each starting with "/", without a
// trailing slash. The host mounts the handler at prefix+"/" and does not strip
// the prefix. observer must not be nil.
func NewHandler(prefix string, observer Observer) (http.Handler, error)
```

`NewHandler` errors: `representation: prefix %q must match ^(/[A-Za-z0-9._~-]+)+$`, `representation: observer is required`.

Routes (`http.ServeMux`):

| Pattern | View |
| --- | --- |
| `GET {prefix}/{$}` | overview |
| `GET {prefix}/entities` | entity list, optional query parameter `kind` |
| `GET {prefix}/entities/{kind}/{id}` | entity detail |

### Views

Hrefs:

- entity: `prefix + "/entities/" + url.PathEscape(kind) + "/" + url.PathEscape(id)`
- entities of a kind: `prefix + "/entities?kind=" + url.QueryEscape(kind)`
- all entities: `prefix + "/entities"`

Overview: kinds in order of first appearance in `Entities()`, with the number of entities of each kind.

```json
{
  "observed_at": "2026-01-02T03:04:05Z",
  "entities_href": "/inspector/entities",
  "kinds": [
    {"kind": "book", "count": 2, "href": "/inspector/entities?kind=book"},
    {"kind": "author", "count": 1, "href": "/inspector/entities?kind=author"}
  ]
}
```

Entity list: entities in snapshot order. With a non-empty `kind` parameter, only entities of that kind; an unknown kind gives an empty list. `state` is omitted when empty.

```json
{
  "observed_at": "2026-01-02T03:04:05Z",
  "entities": [
    {"kind": "book", "id": "b1", "state": "available", "href": "/inspector/entities/book/b1"},
    {"kind": "book", "id": "b2", "state": "lent", "href": "/inspector/entities/book/b2"},
    {"kind": "author", "id": "a1", "href": "/inspector/entities/author/a1"}
  ]
}
```

Entity detail: `related` is `navigation.Links` in its order.

```json
{
  "observed_at": "2026-01-02T03:04:05Z",
  "entity": {
    "kind": "book", "id": "b1", "state": "available", "href": "/inspector/entities/book/b1",
    "attributes": [{"name": "title", "value": "Dune"}]
  },
  "related": [
    {"relation": "written_by", "direction": "outgoing",
     "target": {"kind": "author", "id": "a1", "href": "/inspector/entities/author/a1"}},
    {"relation": "cites", "direction": "outgoing",
     "target": {"kind": "book", "id": "b2", "href": "/inspector/entities/book/b2"}}
  ]
}
```

Lists are never `null`: `kinds`, `entities`, `attributes` and `related` encode as `[]` when empty.

Entity detail sequence: observe; `links, err := navigation.Links(snap, ref)`; `errors.Is(err, navigation.ErrUnknownEntity)` gives 404; any other error gives 500; then `entity, _ := snap.Entity(ref)` (it exists, `Links` succeeded).

### Responses and errors

`Content-Type: application/json; charset=utf-8`, body followed by `\n`. Errors have the form `{"error":{"code":"<code>","message":"<text>"}}`:

| Condition | Status | Code | Message |
| --- | --- | --- | --- |
| `Observe` returns an error | 503 | `source_unavailable` | the observer error text |
| entity not in snapshot | 404 | `entity_not_found` | `entity <kind>/<id> not found` |
| `json.Marshal` of a view fails (for example `observed_at` year 10000) | 500 | `internal_error` | the marshal error text |
| method other than GET | 405 | set by `http.ServeMux` | set by `http.ServeMux` |

The handler does not log. Errors are visible in the response.

### Resource types (`resources.go`)

```go
type overviewResource struct {
    ObservedAt   time.Time      `json:"observed_at"`
    EntitiesHref string         `json:"entities_href"`
    Kinds        []kindResource `json:"kinds"`
}

type kindResource struct {
    Kind  string `json:"kind"`
    Count int    `json:"count"`
    Href  string `json:"href"`
}

type entityListResource struct {
    ObservedAt time.Time         `json:"observed_at"`
    Entities   []entitySummary   `json:"entities"`
}

type entitySummary struct {
    Kind  string `json:"kind"`
    ID    string `json:"id"`
    State string `json:"state,omitempty"`
    Href  string `json:"href"`
}

type entityDetailResource struct {
    ObservedAt time.Time         `json:"observed_at"`
    Entity     entityResource    `json:"entity"`
    Related    []relatedResource `json:"related"`
}

type entityResource struct {
    Kind       string              `json:"kind"`
    ID         string              `json:"id"`
    State      string              `json:"state,omitempty"`
    Href       string              `json:"href"`
    Attributes []attributeResource `json:"attributes"`
}

type attributeResource struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type relatedResource struct {
    Relation  string      `json:"relation"`
    Direction string      `json:"direction"`
    Target    refResource `json:"target"`
}

type refResource struct {
    Kind string `json:"kind"`
    ID   string `json:"id"`
    Href string `json:"href"`
}

type errorResource struct {
    Error errorBody `json:"error"`
}

type errorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### `doc.go`

1. First sentence: `Package representation is the Representation domain of Inspector: it presents observed snapshots in forms that humans and tools can consume.`
2. Core questions from the deleted `README.md`: How should this be presented? How can understanding be communicated?
3. `# Endpoints`: the route table.
4. `# Freshness`: one `Observe` call per request; no caching.
5. `# JSON`: snake_case names, hrefs, lists never null, error format and codes.
6. `# Mounting`: the host mounts at `prefix+"/"` without stripping the prefix.

### Root `README.md` row

```markdown
| `representation` | Representation | JSON views of snapshots over HTTP with navigable hrefs. | `observation`, `navigation` |
```

### Tests (`handler_test.go`, package `representation_test`)

Fixture: the neutral snapshot of step 02 (`b1`, `b2`, `a1`, relations `b1 -written_by-> a1`, `b2 -written_by-> a1`, `b1 -cites-> b2`, observed at `2026-01-02T03:04:05Z`). `stubObserver` returns a fixed snapshot or error and counts calls. Requests run through `httptest.NewRecorder`; no server is started. Bodies are compared with `require.JSONEq`.

| Test | Assertion |
| --- | --- |
| `TestNewHandlerRejectsInvalidConfig` | Prefixes `""`, `/`, `inspector`, `/inspector/`, `/in spector`, `/in{x}` each return an error; `nil` observer returns an error. |
| `TestOverview` | `GET /inspector/` is 200 and JSON-equal to the overview example. |
| `TestEntityList` | `GET /inspector/entities` is 200 and JSON-equal to the list example. |
| `TestEntityListFilteredByKind` | `?kind=book` lists `b1`, `b2` only. `?kind=missing` gives `"entities": []`. |
| `TestEntityDetail` | `GET /inspector/entities/book/b1` is 200 and JSON-equal to the detail example. |
| `TestEntityDetailIncomingAndEmpty` | `author/a1`: no `state` key, `attributes` `[name=Frank Herbert]`, `related` `[written_by incoming b1, written_by incoming b2]`. `book/b2`: `attributes` is `[]`. |
| `TestEntityDetailEscapedID` | Snapshot with entity `book` / `a/b c`: the list shows href `/inspector/entities/book/a%2Fb%20c`; `GET` of that href is 200 with `"id": "a/b c"`. |
| `TestEntityNotFound` | `GET /inspector/entities/book/zz` is 404 with code `entity_not_found` and message `entity book/zz not found`. |
| `TestSourceUnavailable` | Observer error `boom`: each of the three routes is 503 with code `source_unavailable` and a message containing `boom`. |
| `TestEmptySnapshot` | Snapshot without entities: overview has `"kinds": []`, list has `"entities": []`. |
| `TestUnencodableSnapshot` | Snapshot observed at `time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)`: overview is 500 with code `internal_error`. |
| `TestObservesOnEveryRequest` | Two `GET /inspector/` requests call the observer twice. |
| `TestOnlyGET` | `POST /inspector/entities` is 405; the observer is not called. |

## Verification

```powershell
go test ./representation/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./representation/...` exits 0 and runs every test listed above.
- `representation/README.md` does not exist; `representation/doc.go` exists.
- `go list -deps ./representation` lists no package path containing `/harness/`.
- `.go-arch-lint.yml` rule for `representation` is exactly `mayDependOn: [observation, navigation]`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- HTML views and any change to the workbench page.
- Pagination, sorting or search.
- Explanation views.
- Mounting in the workbench (step 07).
