---
title: "05 - Dashboard editing"
dependencies: ["04-dashboard-views"]
effort: "M"
complexity: "medium"
---

# 05 - Dashboard editing

## Objective

Users create and delete dashboards and add and remove panels through HTML forms. The overview, entity list and entity pages offer "add to dashboard" forms, so dashboards are built while navigating. `FileStore` gains `Create`, `Update` and `Delete`, each persisted atomically before memory changes. Form posts change only the dashboards file: they never call the observer and never reach the inspected system. Cross-site browser posts are rejected by `http.CrossOriginProtection`.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/dashboard/store.go` | `ErrDashboardExists`, `ErrDashboardNotFound`, `Create`, `Update`, `Delete`. |
| `representation/dashboard/store_test.go` | Write tests. |
| `representation/dashboard/doc.go` | Section `# Writes`. |
| `representation/handler.go` | `Dashboards` gains `Create`, `Update`, `Delete`; four POST routes; `NewHandler` wraps the mux with `http.NewCrossOriginProtection().Handler`. |
| `representation/forms.go` | New: form handlers, form parsing, store error mapping. |
| `representation/templates/dashboards.html` | Create form. |
| `representation/templates/dashboard.html` | Three add-panel forms, a remove button per panel, a delete button. |
| `representation/templates/layout.html` | Shared template `dashboard-select`. |
| `representation/templates/overview.html`, `entities.html`, `entity.html` | "Add to dashboard" forms. |
| `representation/forms_test.go` | New tests, package `representation_test`. |
| `representation/doc.go` | Section `# Editing`; endpoint table. |
| `harness/README.md` | Inspector endpoint table: four POST rows. |

## Implementation Tasks

1. Add the write tests to `store_test.go`. Confirm they fail. Implement `Create`, `Update`, `Delete`.
2. Write `forms_test.go`. Confirm it fails.
3. Extend `Dashboards`, add `forms.go`, register the routes, wrap the mux.
4. Add the forms to the templates.
5. Update `doc.go` files and `harness/README.md`.
6. `go test -race ./representation/...`, then `task all`.

## Technical Details

### Store writes (`store.go`)

```go
// Errors of FileStore writes.
var (
    ErrDashboardExists   = errors.New("dashboard exists")
    ErrDashboardNotFound = errors.New("dashboard not found")
)

// Create validates d and appends it. It returns an error wrapping
// ErrDashboardExists when the ID is taken.
func (s *FileStore) Create(d Dashboard) error

// Update applies change to a copy of the dashboard with id, validates the
// result and saves it. An error returned by change is returned as is and
// nothing changes. change must not alter the ID; that is an error wrapping
// ErrInvalidDashboard. It returns an error wrapping ErrDashboardNotFound when
// id does not exist.
func (s *FileStore) Update(id string, change func(*Dashboard) error) error

// Delete removes the dashboard with id. It returns an error wrapping
// ErrDashboardNotFound when id does not exist.
func (s *FileStore) Delete(id string) error
```

Every write holds `mu`, builds the new list as a copy, calls `save(newList)` and replaces `s.dashboards` only when `save` succeeded. A failed write leaves the file and memory as they were. Error formats: `dashboard: %w: %s` with the sentinel and the ID, for example `dashboard: dashboard not found: zz`.

### Interface (`handler.go`)

```go
type Dashboards interface {
    List() []dashboard.Dashboard
    Get(id string) (dashboard.Dashboard, bool)
    Create(d dashboard.Dashboard) error
    Update(id string, change func(*dashboard.Dashboard) error) error
    Delete(id string) error
}
```

`NewHandler` returns `http.NewCrossOriginProtection().Handler(mux)`. GET, HEAD and OPTIONS always pass. A POST with `Sec-Fetch-Site: cross-site`, or an `Origin` whose host differs from `Host`, gets 403. A POST without those headers (for example from `curl`) passes.

### Routes and form fields (`forms.go`)

| Route | Form fields | Store call | Success |
| --- | --- | --- | --- |
| `POST {prefix}/ui/dashboards` | `id`, `title` | `Create(Dashboard{ID, Title, Panels: []Panel{}})` | 303 to `h.ui.dashboard(id)` |
| `POST {prefix}/ui/dashboards/{id}/delete` | none | `Delete(id)` | 303 to `h.ui.dashboards()` |
| `POST {prefix}/ui/panels` | `dashboard`, `type`, `kind`, `state`, `entity_id`, `limit` | `Update(dashboard, append panel)` | 303 to `h.ui.dashboard(dashboard)` |
| `POST {prefix}/ui/dashboards/{id}/panels/{index}/delete` | none | `Update(id, remove panel index)` | 303 to `h.ui.dashboard(id)` |

Form parsing: `r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)` with `maxFormBytes = 64 << 10`, then `r.ParseForm()`; values are read with `r.PostFormValue`. The panel is built from every field: `Panel{Type: PanelType(type), Kind: kind, State: state, ID: entity_id, Limit: limit}`. `limit` empty is 0; otherwise `strconv.Atoi`. `Validate` rejects fields that do not belong to the type.

Remove panel: `index` is `strconv.Atoi(r.PathValue("index"))`; a parse error or an index outside the panels makes the change return `errPanelNotFound` (unexported, in `forms.go`). Removal keeps the order of the other panels. Indexes refer to the panel order when the page was rendered; a concurrent edit in another tab can shift them.

Error pages (HTML, rendered by `writePageError`):

| Condition | Status | Code |
| --- | --- | --- |
| body too large or unparsable form; `limit` not a number | 400 | `invalid_form` |
| `errors.Is(err, dashboard.ErrInvalidDashboard)` | 400 | `invalid_dashboard` |
| `errors.Is(err, dashboard.ErrDashboardExists)` | 409 | `dashboard_exists` |
| `errors.Is(err, dashboard.ErrDashboardNotFound)` | 404 | `dashboard_not_found` |
| `errors.Is(err, errPanelNotFound)` | 404 | `panel_not_found` |
| any other store error | 500 | `store_failed` |

The message is `err.Error()`. The handler does not log; the error is visible on the page.

### Forms (templates)

`layout.html` defines a shared template:

```html
{{define "dashboard-select"}}<select name="dashboard" required>{{range .}}<option value="{{.ID}}">{{.Title}}</option>{{end}}</select>{{end}}
```

| Template | Form |
| --- | --- |
| `dashboards.html` | `<form method="post" action="{{.UI}}/dashboards">` with `id` (`required`, `pattern="[a-z0-9][a-z0-9-]{0,62}"`), `title` (`required`, `maxlength="100"`), button `Create dashboard`. |
| `dashboard.html` | Per panel, inside `{{range $i, $p := .View.Panels}}`: `<form method="post" action="{{$.UI}}/dashboards/{{$.View.Dashboard.ID}}/panels/{{$i}}/delete"><button>Remove panel</button></form>`. Three forms posting to `{{.UI}}/panels`, each with hidden `dashboard` and `type`: `states` with `kind`; `entities` with `kind`, `state`, `limit` (`type="number" min="1" max="100" value="10" required`); `entity` with `kind`, `entity_id`. Button `Delete dashboard` posting to `{{.UI}}/dashboards/{{.View.Dashboard.ID}}/delete`. Dashboard IDs are validated slugs, so they are safe in paths. |
| `overview.html` | Per kind row, when `.Dashboards` is not empty: form to `{{$.UI}}/panels` with hidden `type=states`, `kind`, `{{template "dashboard-select" $.Dashboards}}`, button `Add states panel`. |
| `entities.html` | When `.Filter.Kind` is not empty and `.Dashboards` is not empty: form with hidden `type=entities`, `kind`, `state` from `.Filter`, `limit` (`value="10"`), select, button `Add to dashboard`. |
| `entity.html` | When `.Dashboards` is not empty: form with hidden `type=entity`, `kind`, `entity_id` from `.View.Entity`, select, button `Add to dashboard`. |

Pages with a pin form but no dashboards show `<a href="{{.UI}}/dashboards">Create a dashboard</a> to add this.` instead of the form.

### Tests

`representation/dashboard/store_test.go` additions:

| Test | Assertion |
| --- | --- |
| `TestCreatePersists` | `Create` then `Get` finds it; a second `Open` of the same file finds it; the file has `"panels": []` for a dashboard created with nil panels. |
| `TestCreateRejectsDuplicate` | `ErrorIs ErrDashboardExists`; file bytes unchanged. |
| `TestCreateRejectsInvalid` | `ErrorIs ErrInvalidDashboard`; file bytes unchanged. |
| `TestUpdateAppliesChange` | Appending a panel is visible in `Get` and after a second `Open`. |
| `TestUpdateUnknown` | `ErrorIs ErrDashboardNotFound`. |
| `TestUpdateReturnsChangeError` | `change` returns `errBoom`: `ErrorIs errBoom`; file bytes and `List()` unchanged. |
| `TestUpdateChangeWorksOnCopy` | `change` sets `Panels[0].Kind = "x"` then returns an error: `Get` still has the old kind. |
| `TestUpdateRejectsInvalidResult` | `change` appends a panel with empty kind: `ErrorIs ErrInvalidDashboard`; unchanged. |
| `TestUpdateRejectsIDChange` | `change` sets `ID`: `ErrorIs ErrInvalidDashboard`; unchanged. |
| `TestDeletePersists` | `Delete` then `Get` fails; a second `Open` does not find it. |
| `TestDeleteUnknown` | `ErrorIs ErrDashboardNotFound`. |
| `TestFailedWriteKeepsState` | After `Open`, remove the directory with `os.RemoveAll`; `Create` fails with an error that wraps none of the three sentinels; `List()` is unchanged. |
| `TestConcurrentUpdates` | 10 goroutines each append one panel to the same dashboard through `Update`; after `sync.WaitGroup.Wait`, `Get` has 10 panels and a second `Open` too. Run with `-race`. |

`representation/forms_test.go` (helper `post(h, target string, form url.Values)` sets `Content-Type: application/x-www-form-urlencoded`; handlers use `storeWith` from step 04 with the `home` and `empty` dashboards and `dashboardFixture`):

| Test | Assertion |
| --- | --- |
| `TestUICreateDashboard` | `id=ops&title=Ops`: 303, `Location` `/inspector/ui/dashboards/ops`; `Get("ops")` has title `Ops` and no panels. |
| `TestUICreateDashboardRejectsInvalid` | `id=Ops` and `title=%20` each: 400 HTML containing `invalid_dashboard`; `List()` unchanged. |
| `TestUICreateDashboardRejectsDuplicate` | `id=home`: 409, `dashboard_exists`. |
| `TestUIDeleteDashboard` | `/inspector/ui/dashboards/home/delete`: 303 to `/inspector/ui/dashboards`; `Get("home")` fails. `/inspector/ui/dashboards/zz/delete`: 404 `dashboard_not_found`. |
| `TestUIAddPanel` | Table, each posted to `/inspector/ui/panels` with `dashboard=empty`: `type=states&kind=book` gives `Panel{Type: "states", Kind: "book"}`; `type=entities&kind=book&state=lent&limit=10` gives the entities panel; `type=entity&kind=book&entity_id=b2` gives the entity panel. Each: 303 to `/inspector/ui/dashboards/empty`, the last panel equals the expected value. |
| `TestUIAddPanelRejectsInvalid` | `type=states&kind=book&state=lent`: 400 `invalid_dashboard`. `type=entities&kind=book&limit=0`: 400 `invalid_dashboard`. `limit=x`: 400 `invalid_form`. `type=chart&kind=book`: 400 `invalid_dashboard`. `dashboard=zz`: 404 `dashboard_not_found`. `Get("empty")` has no panels afterwards. |
| `TestUIRemovePanel` | `/inspector/ui/dashboards/home/panels/1/delete`: 303; `home` has 4 panels and the former third panel is now second. Indexes `9`, `-1`, `x`: 404 `panel_not_found`. |
| `TestUIWritesRejectCrossSite` | Create with header `Sec-Fetch-Site: cross-site`: 403, `Get("ops")` fails. With `Sec-Fetch-Site: same-origin`: 303. |
| `TestUIWritesDoNotObserve` | Create, add panel, remove panel, delete: `observer.calls` is 0. |
| `TestUIStoreFailure` | Store stub whose writes return `errors.New("disk full")`: create is 500 HTML containing `store_failed` and `disk full`. |
| `TestUIFormTooLarge` | `title` of 70000 bytes: 400 `invalid_form`. |
| `TestUIPinForms` | With `home`: overview contains `action="/inspector/ui/panels"` and `name="type" value="states"`; `/inspector/ui/entities?kind=book&state=lent` contains `name="type" value="entities"` and `name="state" value="lent"`; `/inspector/ui/entities` contains no `value="entities"`; `/inspector/ui/entities/book/b2` contains `name="type" value="entity"` and `name="entity_id" value="b2"`. |
| `TestUIPinFormsWithoutDashboards` | Empty store: entity page contains `Create a dashboard` and does not contain `action="/inspector/ui/panels"`. |
| `TestUIDashboardPageForms` | `/inspector/ui/dashboards/home` contains `action="/inspector/ui/dashboards/home/panels/0/delete"`, `action="/inspector/ui/dashboards/home/delete"`, `name="type" value="states"`, `name="type" value="entities"`, `name="type" value="entity"`. |

## Verification

```powershell
go test -race ./representation/...
task boundary
task deadcode
task all
```

## Acceptance Criteria

- `go test -race ./representation/...` exits 0 and runs every test listed above.
- `representation/handler.go` contains `http.NewCrossOriginProtection()`.
- `representation/forms.go` does not reference `h.observer`.
- `task deadcode` prints `deadcode: no issues found`.
- `task all` exits 0.

## Non-Goals

- Renaming dashboards, reordering or editing panels.
- A JSON write API.
- Undo, history of dashboard changes, file locking between processes.
