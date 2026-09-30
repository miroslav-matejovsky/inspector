---
title: "04 - Dashboard views and store wiring"
dependencies: ["01-dashboard-model-and-store", "03-html-pages"]
effort: "L"
complexity: "medium"
---

# 04 - Dashboard views and store wiring

## Objective

`representation` evaluates dashboards against the current snapshot and serves them as JSON views and HTML pages. A states panel counts the entities of a kind by state, an entities panel lists entities of a kind and optional state, and an entity panel shows one entity with its state, reason and root causes. `NewHandler` takes the dashboards through a small interface. The workbench opens the dashboards file named by a new required flag at startup and passes the store to the inspector.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/handler.go` | Interface `Dashboards`; `NewHandler(prefix, observer, dashboards)`; routes `GET {prefix}/dashboards`, `GET {prefix}/dashboards/{id}`, `GET {prefix}/ui/dashboards`, `GET {prefix}/ui/dashboards/{id}`. |
| `representation/resources.go` | Dashboard resources; `links.dashboards`, `links.dashboard`. |
| `representation/views.go` | `dashboardListOf`, `dashboardViewOf`. |
| `representation/html.go` | Pages `dashboards`, `dashboard`; `page.Dashboards`; `startPage` split into `newPage` and `observePage`. |
| `representation/templates/layout.html` | Navigation link `Dashboards`. |
| `representation/templates/overview.html` | Section `Dashboards` listing `.Dashboards`. |
| `representation/templates/dashboards.html` | New. |
| `representation/templates/dashboard.html` | New. |
| `representation/handler_test.go` | `newHandler` passes an empty store; `TestNewHandlerRejectsInvalidConfig` adds a nil store case. |
| `representation/dashboard_view_test.go` | New tests, package `representation_test`. |
| `representation/doc.go` | Section `# Dashboards`; endpoint table. |
| `.go-arch-lint.yml` | `representation: { mayDependOn: [observation, navigation, explanation, dashboard] }`; `workbench: { mayDependOn: [inspected, adapter, connectivity, representation, dashboard] }`. |
| `harness/workbench/config.go` | `Config.InspectorDashboardsFile`; flag `-inspector-dashboards-file`; validation. |
| `harness/workbench/workbench.go` | `InspectorHandler(inspectedURL, sourceTimeout, dashboards)`; `Run` opens the store. |
| `harness/workbench/config_test.go`, `workbench_test.go`, `scenario_test.go` | New flag; `validConfig(t)`; store passed to `InspectorHandler`; new tests. |
| `harness/workbench/doc.go` | Configuration list and inspector section. |
| `Taskfile.yml` | `workbench` passes `-inspector-dashboards-file .workbench-dashboards.json`. |
| `.gitignore` | `.workbench-dashboards.json` and `.workbench-dashboards.json.tmp`. |
| `README.md` (root) | `representation` row: may depend on `representation/dashboard`. |
| `harness/README.md` | Flag table row; inspector endpoint table: four dashboard rows. |

## Implementation Tasks

1. Write `dashboard_view_test.go` with every representation test listed below; update `handler_test.go`. Confirm the package fails to compile.
2. Add resources, links and builders; then the interface, `NewHandler` and the JSON routes.
3. Add the two pages, the layout link and the overview section; register the UI routes.
4. Update `.go-arch-lint.yml`.
5. Update the workbench tests (config, `validConfig(t)`, store helper, new tests); then `config.go` and `workbench.go`.
6. Update `Taskfile.yml`, `.gitignore`, `doc.go` files, root `README.md`, `harness/README.md`.
7. `go test ./representation/... ./harness/workbench/...`, then `task all`.

## Technical Details

### Interface and constructor (`handler.go`)

```go
// Dashboards provides the dashboards the handler shows. List returns them in
// display order. Both methods return copies the handler may keep.
type Dashboards interface {
    List() []dashboard.Dashboard
    Get(id string) (dashboard.Dashboard, bool)
}

// NewHandler ... dashboards must not be nil.
func NewHandler(prefix string, observer Observer, dashboards Dashboards) (http.Handler, error)
```

New error: `representation: dashboards are required`. `*dashboard.FileStore` satisfies `Dashboards`.

### Links (`resources.go`)

```go
func (l links) dashboards() string          // base + "/dashboards"
func (l links) dashboard(id string) string  // base + "/dashboards/" + url.PathEscape(id)
```

### Resources (`resources.go`)

```go
type dashboardListResource struct {
    Dashboards []dashboardSummary `json:"dashboards"` // never null
}

type dashboardSummary struct {
    ID         string `json:"id"`
    Title      string `json:"title"`
    PanelCount int    `json:"panel_count"`
    Href       string `json:"href"`
}

type dashboardViewResource struct {
    ObservedAt time.Time        `json:"observed_at"`
    Gaps       []gapResource    `json:"gaps"`
    Dashboard  dashboardSummary `json:"dashboard"`
    Panels     []panelResource  `json:"panels"` // never null
}

// panelResource is one evaluated panel. Exactly one of States, Entities and
// Entity is set, the one named by Type.
type panelResource struct {
    Type     string                 `json:"type"`
    States   *statesPanelResource   `json:"states,omitempty"`
    Entities *entitiesPanelResource `json:"entities,omitempty"`
    Entity   *entityPanelResource   `json:"entity,omitempty"`
}

type statesPanelResource struct {
    Kind   string               `json:"kind"`
    Total  int                  `json:"total"`
    Href   string               `json:"href"`   // entities of the kind
    Counts []stateCountResource `json:"counts"` // never null
}

type stateCountResource struct {
    State string `json:"state,omitempty"`
    Count int    `json:"count"`
    Href  string `json:"href,omitempty"` // empty for the empty state: the list cannot filter by it
}

type entitiesPanelResource struct {
    Kind     string          `json:"kind"`
    State    string          `json:"state,omitempty"`
    Limit    int             `json:"limit"`
    Total    int             `json:"total"`    // matching entities, before the limit
    Href     string          `json:"href"`     // entity list with the same filters
    Entities []entitySummary `json:"entities"` // never null, at most Limit
}

type entityPanelResource struct {
    Kind            string              `json:"kind"`
    ID              string              `json:"id"`
    Observed        bool                `json:"observed"`
    State           string              `json:"state,omitempty"`
    Reason          string              `json:"reason,omitempty"`
    Href            string              `json:"href,omitempty"`             // set when observed
    ExplanationHref string              `json:"explanation_href,omitempty"` // set when observed
    RootCauses      []rootCauseResource `json:"root_causes"`                // never null
}
```

### Evaluation (`views.go`)

```go
// dashboardListOf lists the dashboards in the given order.
func dashboardListOf(ds []dashboard.Dashboard, l links) dashboardListResource

// dashboardViewOf evaluates every panel of d against s, in panel order.
func dashboardViewOf(s observation.Snapshot, l links, d dashboard.Dashboard) (dashboardViewResource, error)
```

| Panel type | Evaluation |
| --- | --- |
| `states` | Entities with `Ref.Kind == Kind`, grouped by `State` in order of first appearance. `Href` of a count: `l.entitiesOf(Kind, State)`; empty when `State` is empty. `Total`: number of entities of the kind. `Href` of the panel: `l.entitiesOf(Kind, "")`. A kind without entities gives `total` 0 and `counts` `[]`. |
| `entities` | Entities with `Ref.Kind == Kind` and, when `State` is not empty, `State == State`, in snapshot order. `Total`: all matches. `Entities`: the first `Limit` matches. `Href`: `l.entitiesOf(Kind, State)`. |
| `entity` | `snap.Entity(ref)`. Not found: `Observed` false, `RootCauses` `[]`, no hrefs. Found: state, reason, `l.entity(ref)`, `l.explanation(ref)`, and `rootCausesOf(s, l, e)` with `e` from `explanation.Explain(s, ref)`. An error from `Explain` is returned wrapped (the entity exists, so it indicates a bug) and becomes a 500. |

The dashboard never fails because a kind, state or entity is missing: dashboards outlive snapshots.

### Routes and errors

| Pattern | Flow |
| --- | --- |
| `GET {prefix}/dashboards` | `dashboardListOf(h.dashboards.List(), h.api)`. Does not observe. |
| `GET {prefix}/dashboards/{id}` | `Get(id)`; missing: 404 `dashboard_not_found`, message `dashboard <id> not found`, without observing. Then observe (503 `source_unavailable`), then `dashboardViewOf`. |
| `GET {prefix}/ui/dashboards` | Page `dashboards`, title `Dashboards`, `View` is `dashboardListOf(..., h.ui)`. Does not observe. |
| `GET {prefix}/ui/dashboards/{id}` | Same flow as JSON; page `dashboard`, title is the dashboard title. |

### Pages (`html.go`, templates)

- `newPage(w, r, title) (page, bool)`: parses `refresh`, fills `Title`, `UI`, `Refresh`, `RefreshOn`, `RefreshOff` and `Dashboards` (`dashboardListOf(h.dashboards.List(), h.ui).Dashboards`).
- `observePage(w, r, *page) (observation.Snapshot, bool)`: observes, fills `ObservedAt` and `Gaps`.
- `page` gets `Dashboards []dashboardSummary // every dashboard, for navigation`.
- `parsePages` adds `dashboards` and `dashboard`.

| Template | Content |
| --- | --- |
| `layout.html` | Adds `<a href="{{.UI}}/dashboards">Dashboards</a>` to `nav`. |
| `overview.html` | Section `Dashboards`: `<a href="{{.Href}}">{{.Title}}</a>` per entry of `.Dashboards`; `No dashboards yet.` when empty. |
| `dashboards.html` | Table of `.View.Dashboards`: `<a href="{{.Href}}">{{.Title}}</a>`, id, panel count. `No dashboards yet.` when empty. |
| `dashboard.html` | One `<section>` per panel. `{{with .States}}`: heading `States of <a href="{{.Href}}">{{.Kind}}</a>`, table of counts with `<a href="{{.Href}}">{{.State}}</a>` or `(no state)`, and `total {{.Total}}`. `{{with .Entities}}`: heading link `{{.Kind}}` plus ` in {{.State}}` when set, list of entity links with state, `showing {{len .Entities}} of {{.Total}}`. `{{with .Entity}}`: observed: heading link `{{.Kind}}/{{.ID}}`, state, reason, root cause links, `<a href="{{.ExplanationHref}}">explanation</a>`; not observed: heading text and `Not observed in this snapshot.` No panels: `This dashboard has no panels.` |

### Workbench

`config.go`:

```go
InspectorDashboardsFile string // path of the JSON file that keeps the inspector dashboards; its directory must exist
```

Flag: `fs.StringVar(&cfg.InspectorDashboardsFile, "inspector-dashboards-file", "", "path of the JSON file that keeps the inspector dashboards, for example .workbench-dashboards.json (required)")`. `Validate`: empty gives `workbench: inspector dashboards file is required`.

`workbench.go`:

```go
// InspectorHandler builds the Inspector views and pages of the inspected
// service at inspectedURL, with the dashboards of dashboards.
func InspectorHandler(inspectedURL string, sourceTimeout time.Duration, dashboards representation.Dashboards) (http.Handler, error)
```

`Run`, after `cfg.Validate()` and before `inspected.New`: `store, err := dashboard.Open(cfg.InspectorDashboardsFile)`; on error return `fmt.Errorf("workbench: %w", err)`. The store is passed to `InspectorHandler`. A broken file stops the start before the port is opened.

`Taskfile.yml`, task `workbench`: append `-inspector-dashboards-file .workbench-dashboards.json`. The file is created in the repository root on the first start; `task clean` does not remove it (it removes only `.test-results`, `.cache`, `site`, `bin` and `*.exe`).

### Example (`dashboard_view_test.go`)

Fixture `dashboardFixture(t)`, observed at `2026-01-02T03:04:05Z`:

- entities: `book/b1` `available`; `book/b2` `lent`, reason `borrowed`; `book/b3` `lent`; `author/a1` without state; `member/m1` `active`.
- relations: `b1 -written_by-> a1`; `b2 -lent_to-> m1` (cause); `b3 -lent_to-> m1` (cause).

Store file (helper `storeWith(t, content string) *dashboard.FileStore` writes it to `t.TempDir()` and opens it):

```json
{"dashboards": [
  {"id": "home", "title": "Home", "panels": [
    {"type": "states", "kind": "book"},
    {"type": "states", "kind": "author"},
    {"type": "entities", "kind": "book", "state": "lent", "limit": 1},
    {"type": "entity", "kind": "book", "id": "b2"},
    {"type": "entity", "kind": "book", "id": "b9"}
  ]},
  {"id": "empty", "title": "Empty", "panels": []}
]}
```

Expected `GET /inspector/dashboards/home`:

```json
{
  "observed_at": "2026-01-02T03:04:05Z",
  "gaps": [],
  "dashboard": {"id": "home", "title": "Home", "panel_count": 5, "href": "/inspector/dashboards/home"},
  "panels": [
    {"type": "states", "states": {"kind": "book", "total": 3, "href": "/inspector/entities?kind=book", "counts": [
      {"state": "available", "count": 1, "href": "/inspector/entities?kind=book&state=available"},
      {"state": "lent", "count": 2, "href": "/inspector/entities?kind=book&state=lent"}
    ]}},
    {"type": "states", "states": {"kind": "author", "total": 1, "href": "/inspector/entities?kind=author", "counts": [
      {"count": 1}
    ]}},
    {"type": "entities", "entities": {"kind": "book", "state": "lent", "limit": 1, "total": 2,
      "href": "/inspector/entities?kind=book&state=lent",
      "entities": [{"kind": "book", "id": "b2", "state": "lent", "href": "/inspector/entities/book/b2"}]}},
    {"type": "entity", "entity": {"kind": "book", "id": "b2", "observed": true, "state": "lent", "reason": "borrowed",
      "href": "/inspector/entities/book/b2", "explanation_href": "/inspector/entities/book/b2/explanation",
      "root_causes": [{"kind": "member", "id": "m1", "state": "active", "href": "/inspector/entities/member/m1"}]}},
    {"type": "entity", "entity": {"kind": "book", "id": "b9", "observed": false, "root_causes": []}}
  ]
}
```

### Tests

`representation/dashboard_view_test.go`:

| Test | Assertion |
| --- | --- |
| `TestDashboardList` | `GET /inspector/dashboards` is 200 and `JSONEq` `{"dashboards": [{"id": "home", "title": "Home", "panel_count": 5, "href": "/inspector/dashboards/home"}, {"id": "empty", "title": "Empty", "panel_count": 0, "href": "/inspector/dashboards/empty"}]}`; the observer is not called. |
| `TestDashboardListEmpty` | Empty store: `{"dashboards": []}`. |
| `TestDashboardView` | `GET /inspector/dashboards/home` is 200 and `JSONEq` the example above. |
| `TestDashboardViewWithoutPanels` | `GET /inspector/dashboards/empty`: `"panels": []`. |
| `TestDashboardViewShowsGaps` | Snapshot with a gap: the view has that gap. |
| `TestDashboardNotFound` | `GET /inspector/dashboards/zz`: 404, code `dashboard_not_found`, message `dashboard zz not found`; the observer is not called. |
| `TestDashboardSourceUnavailable` | Observer error `boom`: `GET /inspector/dashboards/home` is 503 `source_unavailable`. |
| `TestUIDashboardListPage` | `/inspector/ui/dashboards`: 200 HTML, contains `href="/inspector/ui/dashboards/home"`, `Home`, `href="/inspector/ui/dashboards"` (nav). |
| `TestUIDashboardPage` | `/inspector/ui/dashboards/home`, `unescaped` body contains `href="/inspector/ui/entities?kind=book&state=lent"`, `(no state)`, `showing 1 of 2`, `href="/inspector/ui/entities/member/m1"`, `href="/inspector/ui/entities/book/b2/explanation"`, `Not observed in this snapshot.` |
| `TestUIDashboardPageWithoutPanels` | `/inspector/ui/dashboards/empty` contains `This dashboard has no panels.` |
| `TestUIOverviewListsDashboards` | `/inspector/ui/` contains `href="/inspector/ui/dashboards/home"`. |
| `TestUIDashboardNotFound` | `/inspector/ui/dashboards/zz`: 404 HTML with `dashboard_not_found`. |

`representation/handler_test.go`: `newHandler` uses a store opened in `t.TempDir()`; `TestNewHandlerRejectsInvalidConfig` also requires an error for `NewHandler("/inspector", &stubObserver{}, nil)`.

`harness/workbench`:

| Test | Assertion |
| --- | --- |
| `TestParseConfig` | `allFlags` gains `{"inspector-dashboards-file", "dashboards.json"}`; the parsed config has `InspectorDashboardsFile: "dashboards.json"`. |
| `TestParseConfigWithoutFlags` | Message: `missing required flags: -addr, -inspected-orders-per-tick, -inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -inspector-dashboards-file, -inspector-source-timeout`. |
| `TestParseConfigRejectsInvalidValues` | New case `"empty dashboards file": {"inspector-dashboards-file": ""}`. |
| `TestInspectorHandlerRejectsInvalidConfig` | Also a nil store gives an error. |
| `TestInspectorServesDashboards` | `InspectorHandler` with an empty store, through `workbench.Handler`: `GET /inspector/dashboards` is 200 `{"dashboards": []}`. |
| `TestRunCreatesDashboardsFile` | `validConfig(t)` with a cancelled context: `Run` returns nil and the dashboards file exists. |
| `TestRunFailsWhenDashboardsDirectoryIsMissing` | `InspectorDashboardsFile` in a missing directory: `Run` returns an error containing `dashboard`. |

`validConfig()` becomes `validConfig(t)` and sets `InspectorDashboardsFile` to `filepath.Join(t.TempDir(), "dashboards.json")`. Every `InspectorHandler` call in `workbench_test.go` and `scenario_test.go` passes a store from a helper `newStore(t)`.

## Verification

```powershell
go test ./representation/... ./harness/workbench/...
go-arch-lint check
task boundary
task deadcode
task all
```

## Acceptance Criteria

- `go test ./representation/... ./harness/workbench/...` exits 0 and runs every test listed above.
- `go list -deps ./representation` includes `github.com/miroslav-matejovsky/inspector/representation/dashboard` and no path containing `/harness/`.
- `Taskfile.yml` task `workbench` contains `-inspector-dashboards-file .workbench-dashboards.json`.
- `.gitignore` contains `.workbench-dashboards.json` and `.workbench-dashboards.json.tmp`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0; `task deadcode` prints `deadcode: no issues found`.

## Non-Goals

- Creating, changing or deleting dashboards (step 05).
- Workbench page changes (step 07).
- Panel types other than `states`, `entities`, `entity`.
