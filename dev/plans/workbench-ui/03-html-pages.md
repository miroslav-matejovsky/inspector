---
title: "03 - HTML pages"
dependencies: ["02-view-builders-and-state-filter"]
effort: "M"
complexity: "medium"
---

# 03 - HTML pages

## Objective

`representation` serves HTML pages for humans under `{prefix}/ui/`: overview, entity list, entity and explanation. Each page renders the resource of the matching JSON view, built by the step 02 builders with the link base `{prefix}/ui`, so every link on a page leads to another page. Pages are rendered on the server with `html/template`; there is no JavaScript. Every page shows the observation time and the gaps, and can reload itself with `?refresh=<seconds>`.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/html.go` | New: embedded templates, `page`, page rendering, HTML errors, refresh parsing, the four page handlers. |
| `representation/templates/layout.html` | New: document, styles, navigation, observation time, refresh link, gaps. |
| `representation/templates/overview.html` | New. |
| `representation/templates/entities.html` | New. |
| `representation/templates/entity.html` | New. |
| `representation/templates/explanation.html` | New. |
| `representation/templates/error.html` | New. |
| `representation/handler.go` | `handler` gets `ui links` (base `prefix + "/ui"`); four new routes. |
| `representation/html_test.go` | New tests, package `representation_test`. |
| `representation/doc.go` | New section `# HTML`; endpoint table. |
| `README.md` (root) | `representation` row: "JSON views and HTML pages of snapshots over HTTP with navigable hrefs." |
| `harness/README.md` | Inspector endpoint table: the four `/inspector/ui/` pages. |

## Implementation Tasks

1. Write `html_test.go` with every test listed below. Confirm it fails.
2. Create the templates, starting with `layout.html` and `error.html`.
3. Create `html.go`; register the routes in `NewHandler`.
4. Update `doc.go`, root `README.md`, `harness/README.md`.
5. `go test ./representation/...`, then `task all`.

## Technical Details

### Routes (added in `NewHandler`)

| Pattern | Page | Title |
| --- | --- | --- |
| `GET {prefix}/ui/{$}` | overview | `Overview` |
| `GET {prefix}/ui/entities` | entities, filters `kind`, `state` | `Entities` |
| `GET {prefix}/ui/entities/{kind}/{id}` | entity | `<kind>/<id>` |
| `GET {prefix}/ui/entities/{kind}/{id}/explanation` | explanation | `Explanation of <kind>/<id>` |

### Rendering (`html.go`)

```go
//go:embed templates/*.html
var templateFS embed.FS

// pages holds one template set per page: templates/layout.html plus the page
// file, which defines "content". A broken template panics at package init,
// so every test run catches it.
var pages = parsePages("overview", "entities", "entity", "explanation", "error")

func parsePages(names ...string) map[string]*template.Template {
    out := make(map[string]*template.Template, len(names))
    for _, name := range names {
        out[name] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html"))
    }
    return out
}

// page is the data of every HTML page. Templates read fields only and never
// call methods of types of this module: task deadcode cannot see calls made
// through reflection.
type page struct {
    Title      string
    UI         string        // base of the pages, prefix + "/ui"
    ObservedAt string        // RFC 3339 in UTC; empty on pages that observe nothing
    Gaps       []gapResource
    Refresh    int           // seconds between reloads; 0 when off
    RefreshOn  string        // this page with refresh=refreshSeconds
    RefreshOff string        // this page without refresh
    Filter     entityFilter  // entity list page: the current filters
    View       any           // the resource of the view
}

// entityFilter holds the filters of the entity list page.
type entityFilter struct{ Kind, State string }

const (
    refreshSeconds    = 5    // interval offered by the refresh link
    maxRefreshSeconds = 3600
)
```

Handler helpers:

```go
// startPage parses refresh, observes and fills the common fields of p. On
// failure it writes an HTML error page and returns false.
func (h *handler) startPage(w http.ResponseWriter, r *http.Request, title string) (page, observation.Snapshot, bool)

// writePage executes the page into a buffer, then writes it with status and
// Content-Type "text/html; charset=utf-8". A template execution error is a 500
// with Content-Type "text/plain; charset=utf-8" and body
// "representation: render <name>: <error>".
func (h *handler) writePage(w http.ResponseWriter, status int, name string, p page)

// writePageError renders the error page with status, code and message.
func (h *handler) writePageError(w http.ResponseWriter, status int, code, message string)
```

Refresh:

- Absent `refresh`: 0. Otherwise `strconv.Atoi`, must be 1 to 3600; else 400 `invalid_refresh`, message `refresh must be a whole number of seconds from 1 to 3600`.
- `RefreshOn`: `r.URL.EscapedPath()` plus the query with `refresh` set to `5`, encoded by `url.Values.Encode`.
- `RefreshOff`: `r.URL.EscapedPath()` plus the query without `refresh`; no `?` when the query is empty.
- `EscapedPath` keeps `%2F` inside IDs; `Path` would break links to entities whose ID contains `/`.

Page flow, for example the entity page: `p, snap, ok := h.startPage(w, r, ref.String())`; `view, err := entityDetailOf(snap, h.ui, ref)`; `errors.Is(err, navigation.ErrUnknownEntity)` renders 404 `entity_not_found` with message `entity <kind>/<id> not found`; other errors render 500 `internal_error`; else `p.View = view` and `writePage(w, 200, "entity", p)`.

Errors are the same as for JSON, rendered by `error.html` with the status: `source_unavailable` 503, `entity_not_found` 404, `internal_error` 500, plus `invalid_refresh` 400.

### Templates

`layout.html` defines `layout`:

```html
{{define "layout"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
{{if .Refresh}}<meta http-equiv="refresh" content="{{.Refresh}}">{{end}}
<title>{{.Title}} - Inspector</title>
<style>
  :root { --bg: #ffffff; --text: #1c1c22; --muted: #6b6b76; --border: #d0d0d7; --link: #2456c7; --warn-bg: #fff4e0; --warn: #8a5200; }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #1f1f25; --text: #ececf1; --muted: #8d8d99; --border: #34343d; --link: #8fb0ff; --warn-bg: #3a2c12; --warn: #f0c070; }
  }
  body { margin: 0; padding: 12px; background: var(--bg); color: var(--text); font: 14px/1.4 system-ui, sans-serif; }
  a { color: var(--link); }
  nav { display: flex; gap: 12px; align-items: baseline; padding-bottom: 8px; border-bottom: 1px solid var(--border); }
  nav .observed { margin-left: auto; color: var(--muted); }
  table { border-collapse: collapse; margin: 8px 0; }
  th, td { padding: 2px 8px; border-bottom: 1px solid var(--border); text-align: left; }
  .gaps { background: var(--warn-bg); color: var(--warn); padding: 4px 12px; margin: 8px 0; }
  .muted { color: var(--muted); }
</style>
</head>
<body>
<nav>
  <a href="{{.UI}}/">Overview</a>
  <a href="{{.UI}}/entities">Entities</a>
  <span class="observed">{{if .ObservedAt}}observed {{.ObservedAt}}{{end}}</span>
  {{if .Refresh}}<a href="{{.RefreshOff}}">stop auto-refresh</a>{{else}}<a href="{{.RefreshOn}}">auto-refresh every 5 s</a>{{end}}
</nav>
{{if .Gaps}}<section class="gaps"><h2>Incomplete observation</h2><ul>{{range .Gaps}}<li><code>{{.Source}}</code>: {{.Error}}</li>{{end}}</ul></section>{{end}}
<main>
<h1>{{.Title}}</h1>
{{template "content" .}}
</main>
</body>
</html>{{end}}
```

Each page file defines `content` and reads `.View`:

| Page | Content |
| --- | --- |
| `overview.html` | Table of `.View.Kinds`: `<a href="{{.Href}}">{{.Kind}}</a>` and `{{.Count}}`. Link `<a href="{{.View.EntitiesHref}}">all entities</a>`. Empty: `No entities observed.` |
| `entities.html` | `<form method="get" action="{{.UI}}/entities">` with inputs `kind` (`value="{{.Filter.Kind}}"`), `state` (`value="{{.Filter.State}}"`) and a submit button. `{{len .View.Entities}} entities`. Table: kind, `<a href="{{.Href}}">{{.ID}}</a>`, state. |
| `entity.html` | State and reason. `<a href="{{.View.ExplanationHref}}">Why is it in this state?</a>`. Attributes table or `No attributes.` History table (from, to, at, reason) or `No recorded history.` Related table: relation, direction, `cause` when `.Cause`, `<a href="{{.Target.Href}}">{{.Target.Kind}}/{{.Target.ID}}</a>`. |
| `explanation.html` | Entity link, state, reason. Root causes: `<a href="{{.Href}}">{{.Kind}}/{{.ID}}</a> {{.State}}` or `No causes reported by the source.` Cause tree via a recursive `cause` template: relation, link, state, reason; `(shown above)` when `.Repeated`; nested `<ul>` of `.Causes` otherwise. History table of the explained entity. |
| `error.html` | `<p><code>{{.View.Code}}</code>: {{.View.Message}}</p>` and `<a href="{{.UI}}/">Back to overview</a>`. `.View` is an `errorBody`. |

`html/template` escapes every value by context. Source text such as `<script>` in an ID is rendered as text.

### `doc.go`, section `# HTML`

Routes; pages render the JSON view resources with hrefs under `{prefix}/ui`; `?refresh=` from 1 to 3600 seconds; HTML error pages with the same codes plus `invalid_refresh`; templates are embedded; no JavaScript; all source text is escaped by `html/template`.

### Tests (`html_test.go`, package `representation_test`)

Helpers: `uiGet(t, s, target) *httptest.ResponseRecorder` using `newHandler`; `unescaped(rec) string` returns `html.UnescapeString(rec.Body.String())`. Link assertions use `unescaped`, because `html/template` writes `&` in attributes as `&amp;`. Escaping assertions use the raw body.

| Test | Assertion |
| --- | --- |
| `TestUIOverviewPage` | `fixture`, `/inspector/ui/`: 200; `Content-Type` is `text/html; charset=utf-8`; `unescaped` contains `href="/inspector/ui/entities?kind=book"`, `href="/inspector/ui/entities?kind=author"`, `2026-01-02T03:04:05Z`. |
| `TestUIEntityListPage` | `/inspector/ui/entities?kind=book&state=lent`: contains `href="/inspector/ui/entities/book/b2"`, does not contain `href="/inspector/ui/entities/book/b1"`, contains `value="book"` and `value="lent"`. |
| `TestUIEntityPage` | `/inspector/ui/entities/book/b1`: contains `Dune`, `href="/inspector/ui/entities/author/a1"`, `href="/inspector/ui/entities/book/b2"`, `href="/inspector/ui/entities/book/b1/explanation"`. `/inspector/ui/entities/book/b2`: contains `acquired`, `day 3`, `borrowed`. |
| `TestUIExplanationPage` | `causeFixture`, `/inspector/ui/entities/room/r1/explanation`: contains `heated_by`, `powered_by`, `no power`, `href="/inspector/ui/entities/fuse/f1"`; does not contain `href="/inspector/ui/entities/lamp/l1"`. |
| `TestUIExplanationPageShowsRepeated` | Snapshot `x/a` and `x/b` causing each other: `/inspector/ui/entities/x/a/explanation` contains `(shown above)`. |
| `TestUIEscapesSourceText` | Entity kind `x`, ID `<script>alert(1)</script>`, attribute `note` = `<b>bold</b>`. Raw body of the list page and of the entity page (href taken from the list) contains neither `<script>alert(1)</script>` nor `<b>bold</b>`; the list page contains `&lt;script&gt;alert(1)&lt;/script&gt;`. |
| `TestUIEscapedIDLinks` | Entity `x` / `a/b c`: list page contains `href="/inspector/ui/entities/x/a%2Fb%20c"`; GET of that path is 200 and contains `a/b c`. |
| `TestUIShowsGaps` | Snapshot with gap `{Source: "part-2", Error: "timed out"}`: overview contains `Incomplete observation`, `part-2`, `timed out`. `fixture`: overview does not contain `Incomplete observation`. |
| `TestUIRefresh` | `?refresh=5` contains `<meta http-equiv="refresh" content="5">`. Without `refresh` the body does not contain `http-equiv="refresh"`. `?refresh=0`, `3601`, `-1`, `x` each give 400 with `invalid_refresh`. |
| `TestUIRefreshLinks` | `/inspector/ui/entities?kind=book`: `unescaped` contains `href="/inspector/ui/entities?kind=book&refresh=5"`. `/inspector/ui/entities?kind=book&refresh=5`: contains `href="/inspector/ui/entities?kind=book"` and `stop auto-refresh`. Entity page of `x` / `a/b c`: contains `href="/inspector/ui/entities/x/a%2Fb%20c?refresh=5"`. |
| `TestUIEntityNotFound` | `/inspector/ui/entities/book/zz` and `.../book/zz/explanation`: 404, `text/html`, contains `entity_not_found` and `entity book/zz not found`. |
| `TestUISourceUnavailable` | Observer error `boom`: each of the four pages is 503 and contains `source_unavailable` and `boom`. |
| `TestUIOnlyGET` | `POST /inspector/ui/entities` is 405; the observer is not called. |

## Verification

```powershell
go test ./representation/...
task boundary
task deadcode
task all
```

Manual check (not required by `task all`): `task workbench`, open http://localhost:8080/inspector/ui/ and follow links from the overview to an entity and its explanation.

## Acceptance Criteria

- `go test ./representation/...` exits 0 and runs every test listed above.
- `representation/templates/` contains exactly `layout.html`, `overview.html`, `entities.html`, `entity.html`, `explanation.html`, `error.html`.
- `Select-String -Path representation/templates/*.html -Pattern '<script'` prints nothing.
- Every existing JSON test in `handler_test.go` passes unchanged.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Dashboards (steps 04, 05).
- Changes to the workbench page (step 07).
- JavaScript, client-side routing, external stylesheets, fonts or icons.
