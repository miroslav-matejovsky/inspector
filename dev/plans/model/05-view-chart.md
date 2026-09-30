---
title: "05 - Health chart view"
dependencies: ["01", "02"]
effort: "S"
complexity: "medium"
---

# 05 - Health chart view

## Objective

The package `view/chart` draws graphical charts of a `*model.Model` as inline SVG. It has one chart now: `chart.HealthByKind` draws one horizontal bar per kind, split by health, with the number of entities. It needs no script and no external library. Nothing outside tests calls the package yet; its functions are staged in the deadcode allowlist until step 07.

## Target Artifacts

| File | Change |
| --- | --- |
| `view/chart/doc.go` | new |
| `view/chart/chart.go` | new: `HealthByKind`, `Styles`, geometry |
| `view/chart/chart.html` | new: template `health` |
| `view/chart/chart.css` | new |
| `view/chart/chart_test.go` | new (package `chart_test`) |
| `view/doc.go` | list `view/chart` |
| `.go-arch-lint.yml` | component `view-chart`, may depend on `model` |
| `taskfile/deadcode.ps1` | staged names |
| `README.md` | Library table: row `view/chart` |

## Implementation Tasks

1. Write `view/chart/chart_test.go` with the tests under Verification.
2. Create `chart.go`, `chart.html` and `chart.css` with the contracts under Technical Details, and make the tests pass.
3. Create `view/chart/doc.go` with the text under "Package documentation".
4. In `view/doc.go`, add the list item `//   - view/chart: SVG charts of a model.Model.`
5. In `.go-arch-lint.yml`, add the component `view-chart: { in: view/chart }` and the deps entry `view-chart: { mayDependOn: [model] }`.
6. Add the row `` | `view/chart` | Explain: SVG charts of a model: entity health by kind. | `model` | `` to the root `README.md` Library table.
7. Run `deadcode ./cmd/...` and stage the new names for `view/chart/` in `taskfile/deadcode.ps1`, under the step 01 comment.
8. Run `task all`.

## Technical Details

### API

```go
package chart

// Styles is the CSS of the charts. A page includes view.Styles and then
// Styles, each once.
var Styles template.CSS

// HealthByKind draws one bar per kind, in the order in which the kinds first
// appear in m.Entities(). A bar is split into segments by health, in the
// order ok, degraded, down, unknown, each as wide as its share of the
// entities of the kind; empty segments are left out. The kind is the label
// on the left and the number of its entities is on the right; every
// segment has a tooltip "<kind>: <count> <health>". A legend of the four
// health values follows the chart. A model without entities renders the
// note "no entities" and no SVG.
func HealthByKind(m *model.Model) (template.HTML, error)
```

A failed template execution returns `chart: render health by kind: <err>`.

### Geometry

```go
const (
	labelWidth = 160 // kind labels, left
	barWidth   = 400 // every bar, whatever the number of its entities
	countGap   = 8   // between the bar and the count
	chartWidth = labelWidth + barWidth + 80
	rowHeight  = 24
	barHeight  = 16
)
```

Row `i` has its bar at `y = i*rowHeight + (rowHeight-barHeight)/2`, and its texts on the baseline `y = i*rowHeight + 16`. The SVG height is `rows*rowHeight`. Segment edges come from the running count, so the widths of a bar add up to `barWidth`:

```go
// For a kind with total entities and counts in health order:
x := float64(labelWidth)
seen := 0
for _, c := range counts {
	if c.n == 0 {
		continue
	}
	seen += c.n
	end := labelWidth + barWidth*float64(seen)/float64(total)
	segments = append(segments, segment{Health: c.health, X: num(x), Width: num(end - x), ...})
	x = end
}

// num formats a coordinate for an SVG attribute.
func num(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
```

Every coordinate goes into the template through `num`, so the attributes are plain decimal strings.

### Markup

```html
<div class="view view-chart">
<svg class="health-by-kind" viewBox="0 0 {{.Width}} {{.Height}}" role="img" aria-label="entity health by kind">
{{range .Rows}}<text class="label" x="0" y="{{.TextY}}">{{.Kind}}</text>
{{range .Segments}}<rect class="health-{{.Health}}" x="{{.X}}" y="{{.Y}}" width="{{.Width}}" height="{{.Height}}"><title>{{.Title}}</title></rect>
{{end}}<text class="count" x="{{.CountX}}" y="{{.TextY}}">{{.Total}}</text>
{{end}}</svg>
<p class="legend"><span class="health-ok">ok</span> <span class="health-degraded">degraded</span> <span class="health-down">down</span> <span class="health-unknown">unknown</span></p>
</div>
```

Template data:

```go
type healthChart struct {
	Width, Height string // viewBox size
	Rows          []bar
}

type bar struct {
	Kind          string
	Total         int
	TextY, CountX string
	Segments      []segment
}

type segment struct {
	Health           model.Health
	X, Y, Width, Height string
	Title            string // "<kind>: <count> <health>"
}
```

html/template treats `<title>` as RCDATA, so put only `{{.Title}}` inside it; it is escaped as text.

### CSS

```css
.view-chart svg { display: block; width: 100%; max-width: 640px; height: auto; }
.view-chart text { font: 12px system-ui, sans-serif; fill: currentColor; }
.view-chart text.count { fill: var(--view-muted); }
.view-chart rect { fill: var(--view-health); }
.view-chart .legend span { color: var(--view-health); margin-right: 12px; }
.view-chart .legend span::before { content: "\25A0 "; }
```

`--view-health` comes from the `health-*` classes of `view.Styles`.

### Package documentation

`view/chart/doc.go`:

```go
// Package chart draws charts of a model.Model as inline SVG, rendered with
// html/template so that every label from the inspected system is escaped.
// Charts need no script.
//
// HealthByKind draws one bar per kind of entity, split by health (ok,
// degraded, down, unknown) in proportion to the number of entities; every
// bar has the same length, so kinds with few and with many entities can be
// compared by share.
//
// A page includes view.Styles and Styles; the colors are the health colors
// of view.Styles.
package chart
```

## Verification

Commands:

```powershell
go test ./view/chart/... -v
go-arch-lint check
task all
```

The tests build models with `model.New(entities, nil)`. A helper `widths(t, html) map[string][]float64` reads the `width` of every `rect` with its title prefix (the kind) using the regexp `<rect class="health-(\w+)" x="[\d.]+" y="[\d.]+" width="([\d.]+)"[^>]*><title>([^:]*):`.

| Test | Asserts |
| --- | --- |
| `TestHealthByKindDrawsOneBarPerKind` | kinds `order`, `product`, `order` give the labels `>order<` before `>product<` and two `class="label"` texts |
| `TestHealthByKindSplitsBarsByHealth` | kind `order` with 2 ok and 1 down has the titles `order: 2 ok` and `order: 1 down`, and no `degraded` or `unknown` rect |
| `TestHealthByKindOrdersSegments` | ok comes before degraded, degraded before down, and down before unknown |
| `TestHealthByKindFillsBars` | for kinds with 1, 3 and 7 entities of mixed health, the widths of each kind add up to 400 within 0.05 |
| `TestHealthByKindShowsTotals` | kind with 3 entities has `<text class="count" ...>3</text>` |
| `TestHealthByKindShowsLegend` | the output contains the four legend spans |
| `TestHealthByKindShowsEmptyModel` | `New(nil, nil)` renders `<p class="note">no entities</p>` and no `<svg` |
| `TestHealthByKindEscapesKinds` | kind `<b>` appears as `&lt;b&gt;` in the label and the title, and `<b>` does not appear |
| `TestHealthByKindMarksRoot` | the output starts with `<div class="view view-chart">` |
| `TestStylesColorBarsByHealth` | `Styles` contains `.view-chart rect { fill: var(--view-health); }` |

## Acceptance Criteria

- Every test in the table passes.
- `go list -f '{{join .Imports " "}}' ./view/chart` prints only standard library packages and `github.com/miroslav-matejovsky/inspector/model`.
- `.go-arch-lint.yml` has `view-chart` with `mayDependOn: [model]`, and `go-arch-lint check` passes.
- `view/doc.go` and the root `README.md` list `view/chart`.
- `task all` passes.

## Non-Goals

- No time series or history charts: the model has the latest signals only.
- No chart library, no script and no interaction beyond SVG tooltips.
- No other chart type.
