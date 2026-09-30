---
title: "07 - Workbench pages"
dependencies: ["02", "03", "04", "05", "06"]
effort: "L"
complexity: "medium"
---

# 07 - Workbench pages

## Objective

The workbench shows the inspected model on three pages, each with tabs in the panel header:

| Tab | Path | View |
| --- | --- | --- |
| Dashboard | `/` | `dashboard.Render` of the inspected model |
| Model | `/model` | `explorer.Index`; with the query parameter `entity`, `explorer.Entity` (404 when the entity is not in the model) |
| Raw | `/raw` | `raw.Render`, as before |

Two more changes:

- The in-place refresh reloads the current page, including its query.
- The dependency controls redirect back to the page they were used on, through a validated `return` form value.

Every function staged by steps 01, 03, 04, 05 and 06 becomes reachable, and the deadcode allowlist is empty again.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/workbench.go` | pages, tabs, `servePage`, model building, styles |
| `harness/workbench/controls.go` | `returnPath`, redirect to it |
| `harness/workbench/index.html` | tabs, hidden `return` input, refresh of the current URL |
| `harness/workbench/workbench_test.go` | page, tab, return and model tests; existing tests adapted |
| `harness/workbench/doc.go` | sections `# Page` and `# Controls` rewritten as `# Pages` and `# Controls` |
| `harness/README.md` | paragraphs `Page.` and `Controls.` rewritten |
| `README.md` | Harness paragraph |
| `.go-arch-lint.yml` | `workbench.mayDependOn` |
| `taskfile/deadcode.ps1` | allowlist emptied |
| `.todo` | live verification items |

## Implementation Tasks

1. Adapt the existing tests in `workbench_test.go`:
   - `page(t, summaries...)` becomes `page(t, path string, summaries ...source.TargetSummary)`.
   - `TestPageShowsRawView`, `TestPageEscapesSignals` and `TestPageShowsCollectedSignals` get `/raw`. Rename `TestPageShowsRawView` to `TestRawPageShowsRawView`.
   - `TestPageShowsSummaryError` becomes a table over `/`, `/model` and `/raw`.
   - `TestPageShowsControlsError` requires `class="view view-dashboard"` instead of `class="view-raw"`.
   - `postMode(h, dependency, mode)` becomes `postMode(h, dependency, mode, ret string)` and sends the form value `return`.
2. Write the new tests under Verification. At this point they fail.
3. Change `workbench.go`, `controls.go` and `index.html` as described under Technical Details, and make the tests pass.
4. Set `workbench.mayDependOn` in `.go-arch-lint.yml` to `[inspected, inspected-fulfillment, source, model, view, view-raw, view-explorer, view-chart, view-dashboard]`.
5. Remove every entry and the comment `# staged by dev/plans/model, removed in step 07` from `$allow` in `taskfile/deadcode.ps1`, so that it reads `$allow = @(` and `)` again. Run `deadcode ./cmd/...`; it must print nothing.
6. Update the documentation with the texts under "Documentation".
7. Add the items under ".todo" to the root `.todo`.
8. Run `task all`.

## Technical Details

### Pages and routes

```go
// Paths of the pages, in tab order.
const (
	pathDashboard = "/"
	pathModel     = "/model"
	pathRaw       = "/raw"
)

// tabs are the pages of the workbench, in tab order.
var tabs = []tab{{Path: pathDashboard, Label: "Dashboard"}, {Path: pathModel, Label: "Model"}, {Path: pathRaw, Label: "Raw"}}

// tab is one page in the panel header. Active marks the page shown.
type tab struct {
	Path, Label string
	Active      bool
}

// pageView is what a page shows below the header.
type pageView struct {
	View   template.HTML
	Status int    // HTTP status of the page
	Error  string // shown instead of View when not empty
}

// renderView renders the view of a page from the summaries of the Source.
// An error means the view could not be rendered.
type renderView func(r *http.Request, summaries []source.TargetSummary) (pageView, error)
```

Routes in `Handler`:

```go
mux.HandleFunc("GET /{$}", servePage(pathDashboard, renderDashboard))
mux.HandleFunc("GET "+pathModel, servePage(pathModel, renderModel))
mux.HandleFunc("GET "+pathRaw, servePage(pathRaw, renderRaw))
mux.HandleFunc("POST /controls/dependencies/{name}", ...) // as before
mux.Handle(inspected.PathPrefix+"/", inspectedHandler)    // as before
```

`servePage(path, render) http.HandlerFunc` is a closure inside `Handler`, next to the template variable `page`, which keeps its name. It keeps the current behavior of `GET /`:

1. Read the dependency controls. On an error, log `workbench: read dependency modes`, set status 500 and set `ControlsError`.
2. Read `signals.Summary`. When that succeeds, call `render`. When either fails, log `workbench: read signals`, set status 500 and set `Error` to `signals unavailable: <err>`.
3. Otherwise take `Status`, `View` and `Error` from the `pageView`. The page status is the larger of the controls status (200 or 500) and `pageView.Status`.
4. Fill the page data and execute the template, as today.

The page data:

```go
type pageData struct {
	RefreshSeconds int
	Tabs           []tab    // copy of tabs, Active set for path
	Return         string   // r.URL.RequestURI(): the page to return to after a control
	Controls       []dependencyControl
	ControlsError  string
	Styles         []template.CSS // view, raw, explorer, chart, dashboard
	View           template.HTML
	Error          string
}
```

The renderers:

```go
func renderDashboard(_ *http.Request, s []source.TargetSummary) (pageView, error) {
	v, err := dashboard.Render(model.Build(s, inspectedInterpreter), pathModel)
	return pageView{View: v, Status: http.StatusOK}, err
}

// renderModel shows the explorer index, or the entity of the query parameter
// entity. An entity that is not in the model is a 404 page, not an error:
// an order may be pruned between two refreshes.
func renderModel(r *http.Request, s []source.TargetSummary) (pageView, error) {
	m := model.Build(s, inspectedInterpreter)
	id := r.URL.Query().Get("entity")
	if id == "" {
		v, err := explorer.Index(m, pathModel)
		return pageView{View: v, Status: http.StatusOK}, err
	}
	if _, ok := m.Entity(id); !ok {
		return pageView{Status: http.StatusNotFound, Error: fmt.Sprintf("entity %q is not in the model", id)}, nil
	}
	v, err := explorer.Entity(m, id, pathModel)
	return pageView{View: v, Status: http.StatusOK}, err
}

func renderRaw(_ *http.Request, s []source.TargetSummary) (pageView, error) {
	v, err := raw.Render(s)
	return pageView{View: v, Status: http.StatusOK}, err
}
```

A 404 entity page is not logged: nobody needs to act on it.

### Return path

`controls.go`:

```go
// returnPath returns the path and query of the page to go back to after a
// control: the form value "return" when its path is a page of the workbench.
// Anything else, such as another host or scheme, is an error, so that the
// redirect never leaves the workbench.
func returnPath(r *http.Request) (string, error) {
	ret := r.FormValue("return")
	u, err := url.Parse(ret)
	if err != nil || u.Scheme != "" || u.Host != "" ||
		!slices.ContainsFunc(tabs, func(t tab) bool { return t.Path == u.Path }) {
		return "", fmt.Errorf("return %q is not a workbench page", ret)
	}
	return u.RequestURI(), nil
}
```

`setDependencyMode` calls `returnPath` first. On an error it answers `http.Error(w, err.Error(), http.StatusBadRequest)` and does not call inspected. On success it redirects to the return path with 303 instead of `/`.

### Template

In `index.html`:

- After `<h2>Workbench</h2>`, add `<nav class="tabs">{{range .Tabs}}<a href="{{.Path}}"{{if .Active}} class="active" aria-current="page"{{end}}>{{.Label}}</a>{{end}}</nav>`.
- In every control form, add `<input type="hidden" name="return" value="{{$.Return}}">` before the buttons.
- Replace `<style>{{.Styles}}</style>` with `{{range .Styles}}<style>{{.}}</style>{{end}}`, if step 02 has not done it already.
- In the script, `refresh` fetches `location.pathname + location.search` instead of `"/"`. Update the script comment: "The panel is replaced in place with the panel of a fresh GET of the current page ...".
- Add CSS for `.tabs`: `display: flex; gap: 4px`. Links get `padding: 2px 10px; border-radius: 4px; color: var(--muted); text-decoration: none`. `.tabs a.active` gets `background: var(--bg); color: var(--text); font-weight: 600`.

### Documentation

Replace the doc comment of `Handler` with this: "Handler serves the pages of the workbench, see the package documentation (# Pages), the dependency controls at "/controls/dependencies/{name}", and every path under inspected.PathPrefix+"/" through inspectedHandler, without stripping the prefix. Failed reads of signals or dependency modes are logged to logger and shown on the page with status 500."

`harness/workbench/doc.go`, section `# Pages`, which replaces `# Page`:

```go
// # Pages
//
// The panel header has a tab per page; the tab of the page shown is marked.
//
//	/                 dashboard.Render of the inspected model
//	/model            explorer.Index of the inspected model
//	/model?entity=ID  explorer.Entity of the entity ID; 404 when it is not in the model
//	/raw              raw.Render of the target summaries of the Source
//
// Every page builds the inspected model, or reads the summaries, when it is
// rendered, and includes the styles of every view. When the summaries cannot
// be read or a view cannot be rendered, the page shows the error instead,
// with status 500.
//
// Every 2 seconds a script of the page gets the current page again and
// replaces the panel in place, so the page does not reload. The scroll
// positions of the elements with a data-scroll key are kept. A failed
// refresh is shown in the panel header and the old panel stays. Without
// JavaScript, the page reloads itself every 2 seconds instead.
```

In `# Controls`, replace "redirects back to the page with 303" with: "redirects with 303 to the page given by the form value return, which every form carries. A return value that is not the path of a workbench page, for example another host, is rejected with 400 before inspected is called."

`harness/README.md`, the paragraph `Page.` is replaced by:

> Pages. The panel header has three tabs. **Dashboard** (`/`) shows the health of the modeled entities by kind as a bar chart, the entities that are degraded or down with the reason inspected states, and the model issues. **Model** (`/model`) lists the entities by kind; a name opens the entity page (`/model?entity=<id>`) with its state, properties, the signal it was read from, its links and its backlinks, each linked to its own page. **Raw** (`/raw`) shows the raw view of package `view/raw`: a full-width table with one row per target, and below it the latest signal of each target with its body, in two columns (one column on screens up to 1100px wide). JSON bodies are indented and highlighted, metrics are highlighted, other bodies are plain text; at most 500 lines per body are shown. Every 2 seconds the current page is refreshed in place, without a page reload, and scroll positions are kept.
>
> Model. The workbench builds the model from the collected signals of `/inspected/`, `/inspected/health/ready`, `/inspected/api/dependencies`, `/inspected/api/products` and `/inspected/api/orders`. Entities are the service, the readiness report and its checks, the dependencies, the products and the orders (newest first). The links are the ones inspected states: readiness, check, determined by (check causes), product, waits on and decided by (order history causes). `task workbench` reads the index, both health endpoints, the metrics and the three API collections. It does not read `/inspected/sim`.

In the paragraph `Controls.`, replace "and refreshes the panel" with "and returns to the page it was used on".

In the root `README.md` Harness paragraph, replace "shows the collected signals in the raw view of package `view/raw`" with "shows a model of the inspected system built from the collected signals (dashboard, entity explorer) and the collected signals in the raw view".

### .todo

Add under a heading `## Model pages (dev/plans/model)`:

```markdown
- [ ] `task workbench`, open http://localhost:8080: the Dashboard tab is active and the chart has bars for service, readiness, check, dependency, product and order.
- [ ] Click `outage` for payment-gateway on the Dashboard: within 10 s, Attention lists the dependency and the check payment-gateway as down with their reasons; the page stays on the Dashboard.
- [ ] Model tab: kinds listed; the first order rows are the newest orders.
- [ ] Open the payment-gateway dependency page: backlinks list the check (determined by) and pending orders (waits on); following a link opens that entity; browser Back returns.
- [ ] On an entity page, click a control: the page stays on the same entity page and the button is filled.
- [ ] Stay on an entity page for 10 s: the panel refreshes in place, the URL keeps `?entity=`, and the scroll position stays.
- [ ] Open `/model?entity=/nope`: the page shows "entity "/nope" is not in the model".
- [ ] Health badges, chart bars and tabs are readable in light and dark mode.
- [ ] With `-inspected-orders-per-tick 10` for 2 minutes (more than 1000 orders), the Model page shows 100 order rows with "more not shown" and refreshes without visible lag.
```

## Verification

Commands:

```powershell
go test ./harness/workbench/... -v
deadcode ./cmd/...
go-arch-lint check
task all
```

The helper `collectInspected(t)` starts an inspected app, serves it with `httptest.NewServer`, opens a `source.Source` with the five interpreted targets of step 03 plus `health_live`, runs `Collect` once, and returns the app handler and the Source. It is built like `TestPageShowsCollectedSignals`.

| Test | Asserts |
| --- | --- |
| `TestDashboardPageShowsInspectedModel` | `GET /` with `collectInspected` answers 200 and contains `class="view view-dashboard"` and `<svg` |
| `TestDashboardPageShowsOutage` | outage of payment-gateway before `Collect`; `GET /` contains `href="/model?entity=%2Finspected%2Fapi%2Fdependencies%2Fpayment-gateway"` |
| `TestModelPageListsKinds` | `GET /model` answers 200 and contains a section heading for each of `service`, `readiness`, `check`, `dependency` and `product` |
| `TestEntityPageShowsEntity` | `GET /model?entity=%2Finspected%2Fapi%2Fdependencies%2Fpayment-gateway` answers 200, contains `payment-gateway` and the backlink type `determined by` |
| `TestEntityPageOfUnknownEntityIsNotFound` | `GET /model?entity=%2Fnope` answers 404 and contains `entity &#34;/nope&#34; is not in the model` |
| `TestRawPageShowsRawView` | `GET /raw` contains the output of `raw.Render` (renamed from `TestPageShowsRawView`) |
| `TestPagesMarkActiveTab` | for `/`, `/model`, `/model?entity=%2Fx` and `/raw`: exactly one `aria-current="page"`, on the link to `/`, `/model`, `/model` and `/raw` |
| `TestPagesIncludeEveryStyle` | `GET /` contains `view.Styles`, `raw.Styles`, `explorer.Styles`, `chart.Styles` and `dashboard.Styles` |
| `TestPagesCarryReturnPath` | `GET /model?entity=%2Fx` has `name="return" value="/model?entity=%2Fx"` in each control form |
| `TestDependencyControlReturnsToPage` | `postMode(h, "payment-gateway", "outage", "/model?entity=%2Fx")` answers 303 with `Location` `/model?entity=%2Fx` |
| `TestDependencyControlRejectsInvalidReturn` | for `""`, `https://evil.test/`, `//evil.test/`, `/elsewhere` and `/\evil.test`: 400, a body with `is not a workbench page`, and `GET /inspected/api/dependencies/payment-gateway` still shows `"mode":"healthy"` |
| `TestPageShowsSummaryError` (changed) | for `/`, `/model` and `/raw`: 500 and `signals unavailable: boom` |
| existing tests | pass with the adaptations of task 1 |

## Acceptance Criteria

- Every test in the table passes.
- `deadcode ./cmd/...` prints nothing, and `$allow` in `taskfile/deadcode.ps1` has no entries.
- `.go-arch-lint.yml` `workbench.mayDependOn` is exactly the list of task 4, and `go-arch-lint check` passes.
- `harness/workbench/doc.go` has the sections `# Inspected model`, `# Pages` and `# Controls` as described; `harness/README.md` has the paragraphs `Pages.`, `Model.` and `Controls.` as described.
- `.todo` has the items of ".todo".
- `task all` passes.

## Non-Goals

- No new flag or configuration: the pages are fixed and the interpreted paths are the ones of package inspected.
- No change to the dependency controls other than the return path.
- No model on the Raw page.
