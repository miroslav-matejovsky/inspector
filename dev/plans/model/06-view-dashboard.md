---
title: "06 - Dashboard view"
dependencies: ["01", "02", "05"]
effort: "S"
complexity: "low"
---

# 06 - Dashboard view

## Objective

The package `view/dashboard` composes an overview of a `*model.Model`. `dashboard.Render` shows three sections:

- the health chart of `view/chart`;
- the entities that need attention (down, then degraded), each linked to its entity page;
- the model issues, when there are any.

Nothing outside tests calls the package yet; its functions are staged in the deadcode allowlist until step 07.

## Target Artifacts

| File | Change |
| --- | --- |
| `view/dashboard/doc.go` | new |
| `view/dashboard/dashboard.go` | new: `Render`, `Styles` |
| `view/dashboard/dashboard.html` | new: template `dashboard` |
| `view/dashboard/dashboard.css` | new |
| `view/dashboard/dashboard_test.go` | new (package `dashboard_test`) |
| `view/doc.go` | list `view/dashboard` |
| `.go-arch-lint.yml` | component `view-dashboard`, may depend on `model` and `view-chart` |
| `taskfile/deadcode.ps1` | staged names |
| `README.md` | Library table: row `view/dashboard` |

## Implementation Tasks

1. Write `view/dashboard/dashboard_test.go` with the tests under Verification.
2. Create `dashboard.go`, `dashboard.html` and `dashboard.css` with the contracts under Technical Details, and make the tests pass.
3. Create `view/dashboard/doc.go` with the text under "Package documentation".
4. In `view/doc.go`, add the list item `//   - view/dashboard: an overview of a model.Model: health chart, entities that need attention, issues.`
5. In `.go-arch-lint.yml`, add the component `view-dashboard: { in: view/dashboard }` and the deps entry `view-dashboard: { mayDependOn: [model, view-chart] }`.
6. Add the row `` | `view/dashboard` | Explain: overview of a model: health chart, degraded and down entities, model issues. | `model`, `view/chart` | `` to the root `README.md` Library table.
7. Run `deadcode ./cmd/...` and stage the new names for `view/dashboard/` in `taskfile/deadcode.ps1`, under the step 01 comment.
8. Run `task all`.

## Technical Details

### API

```go
package dashboard

// maxAttention is the number of entities shown under Attention.
const maxAttention = 50

// Styles is the CSS of the dashboard. A page includes view.Styles,
// chart.Styles and then Styles, each once.
var Styles template.CSS

// Render renders the dashboard of m: the chart.HealthByKind of m; the
// entities with health down and then degraded, each group in model order,
// at most maxAttention, with health badge, kind, name linked to its entity
// page, state value and reason; and the issues of m with their target, when
// there are any. entityPage is the path of the page that shows an entity
// for the query parameter entity.
func Render(m *model.Model, entityPage string) (template.HTML, error)
```

The errors are the error of `chart.HealthByKind`, wrapped as `dashboard: %w`, and `dashboard: render: <err>` for a failed template execution. The entity URL is `entityPage + "?entity=" + url.QueryEscape(id)`. Only entities of the model are listed, so every name is a link.

### Markup

```html
<div class="view view-dashboard">
<section class="health"><h3>Health</h3>{{.Chart}}</section>
<section class="attention"><h3>Attention <span class="count">{{.AttentionCount}}</span></h3>
{{if .Attention}}<table class="attention">
<thead><tr><th>Health</th><th>Kind</th><th>Name</th><th>State</th><th>Reason</th></tr></thead>
<tbody>{{range .Attention}}<tr class="health-{{.Health}}"><td><span class="badge">{{.Health}}</span></td><td>{{.Kind}}</td><td><a href="{{.Href}}">{{.Name}}</a></td><td>{{.Value}}</td><td>{{.Reason}}</td></tr>
{{end}}</tbody></table>{{if .Hidden}}<p class="note">{{.Hidden}} more not shown</p>{{end}}
{{else}}<p class="note">no entity is degraded or down</p>{{end}}
</section>
{{if .Issues}}<section class="issues"><h3>Issues <span class="count">{{len .Issues}}</span></h3>
<ul>{{range .Issues}}<li><span class="target">{{.Target}}</span> <span class="error">{{.Error}}</span></li>{{end}}</ul>
</section>{{end}}
</div>
```

`AttentionCount` is the number of degraded and down entities, including the hidden ones. `Value` and `Reason` are `-` when empty.

### CSS

`dashboard.css` is scoped to `.view-dashboard`. The sections `health` and `attention` sit side by side in a grid, `grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; align-items: start;`, and in one column at `@media (max-width: 1100px)`. The same breakpoint is used by the raw view. `section.issues` spans both columns (`grid-column: 1 / -1`). `table.attention` uses the table style of the explorer tables.

### Package documentation

`view/dashboard/doc.go`:

```go
// Package dashboard renders an overview of a model.Model that explains what
// needs attention: the health of the entities by kind (chart.HealthByKind),
// the entities that are down and then degraded with the reason the system
// states, each linked to its entity page (entityPage?entity=<id>), and the
// issues of the model, the signals that could not be understood. At most 50
// entities are listed, followed by the number left out.
//
// A page includes view.Styles, chart.Styles and Styles.
package dashboard
```

## Verification

Commands:

```powershell
go test ./view/dashboard/... -v
go-arch-lint check
task all
```

The tests build models with `model.New(entities, issues)`.

| Test | Asserts |
| --- | --- |
| `TestRenderShowsHealthChart` | the output contains the exact output of `chart.HealthByKind(m)` |
| `TestRenderListsDownBeforeDegraded` | model order `d1` degraded, `x` ok, `d2` down, `u` unknown: the rows are `d2` then `d1`; `x` and `u` are not listed |
| `TestRenderLinksAttentionToEntityPage` | entity `/deps/pg` gives `href="/model?entity=%2Fdeps%2Fpg"` with entityPage `/model` |
| `TestRenderShowsStateAndReason` | value `outage` and reason `calls fail` are in the row |
| `TestRenderCountsAttention` | 3 down entities give `Attention <span class="count">3</span>` |
| `TestRenderLimitsAttention` | 51 down entities give 50 rows, the count 51 and `<p class="note">1 more not shown</p>` |
| `TestRenderShowsNoAttentionNote` | only ok entities give `<p class="note">no entity is degraded or down</p>` |
| `TestRenderListsIssues` | issue `{products, interpret: status 503}` is shown with its target and error |
| `TestRenderOmitsIssuesWhenNone` | no issues gives no `class="issues"` |
| `TestRenderEscapesValues` | a reason `<i>x</i>` appears as `&lt;i&gt;x&lt;/i&gt;` |
| `TestRenderMarksRoot` | the output starts with `<div class="view view-dashboard">` |
| `TestStylesStyleDashboard` | `Styles` contains `.view-dashboard` and `grid-template-columns: repeat(2, minmax(0, 1fr))` |

## Acceptance Criteria

- Every test in the table passes.
- `go list -f '{{join .Imports " "}}' ./view/dashboard` prints only standard library packages, `github.com/miroslav-matejovsky/inspector/model` and `github.com/miroslav-matejovsky/inspector/view/chart`.
- `.go-arch-lint.yml` has `view-dashboard` with `mayDependOn: [model, view-chart]`, and `go-arch-lint check` passes.
- `view/doc.go` and the root `README.md` list `view/dashboard`.
- `task all` passes.

## Non-Goals

- No configuration of the dashboard sections.
- No history of health changes.
- No explorer index on the dashboard: the Model page shows it.
