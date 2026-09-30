---
title: "02 - Split view, move raw view"
dependencies: []
effort: "S"
complexity: "low"
---

# 02 - Split view, move raw view

## Objective

`view` becomes a family of packages:

- The root package `view` holds only the shared CSS of every view: the palette, the common classes and the health classes that the model views of steps 04 to 06 use.
- The raw view moves unchanged in behavior to `view/raw`. `view.Raw` becomes `raw.Render`, and its CSS becomes `raw.Styles`.
- The root element of the raw view gets the classes `view view-raw`.
- The workbench shows the same page as before.

## Target Artifacts

| File | Change |
| --- | --- |
| `view/raw.go`, `view/raw.html`, `view/body.go`, `view/json.go`, `view/metrics.go`, `view/raw_test.go` | moved to `view/raw/` (same names), package `raw` / `raw_test` |
| `view/view.go` | moved to `view/raw/templates.go`, package `raw`, embeds `raw.css` |
| `view/view.css` | raw-specific rules moved to `view/raw/raw.css`; shared rules stay in a new `view/view.css` |
| `view/raw/doc.go` | new: the raw view part of today's `view/doc.go` |
| `view/view.go` | new: `Styles` of the shared CSS |
| `view/view_test.go` | new: tests of the shared CSS |
| `view/doc.go` | rewritten: the view family and its conventions |
| `harness/workbench/workbench.go`, `harness/workbench/index.html` | use `raw.Render`; render a list of styles |
| `harness/workbench/workbench_test.go` | use `raw.Render` and `raw.Styles` |
| `harness/workbench/doc.go`, `harness/README.md`, `README.md` | package names |
| `.go-arch-lint.yml` | components `view`, `view-raw` |

## Implementation Tasks

1. Move the files with `git mv` (moving is not committing): `view/raw.go`, `view/raw.html`, `view/body.go`, `view/json.go`, `view/metrics.go` and `view/raw_test.go` to `view/raw/`, and `view/view.go` to `view/raw/templates.go`.
2. In every moved `.go` file, change `package view` to `package raw`. In `raw_test.go`, change `package view_test` to `package raw_test` and the import `.../view` to `.../view/raw`.
3. In `view/raw/raw.go`, rename `func Raw` to `func Render`. Keep its doc comment, changed to name `Render`, and change the error prefix from `view: render raw view` to `raw: render`. In `raw_test.go`, replace `view.Raw` with `raw.Render` and `view.Styles` with `raw.Styles`.
4. Write `view/view_test.go` (package `view_test`) and the new raw test `TestRenderMarksViewRoot` (see Verification). At this point they fail.
5. Split `view/view.css` as described under "CSS split": create `view/raw/raw.css` and rewrite `view/view.css`.
6. In `view/raw/templates.go`, embed `raw.css` instead of `view.css` into `var styles`. Keep `var Styles = template.CSS(styles)` with the doc comment: "Styles is the CSS of the raw view. A page that shows the raw view includes view.Styles and then Styles, each once."
7. Create `view/view.go`:

   ```go
   package view

   import (
   	_ "embed"
   	"html/template"
   )

   //go:embed view.css
   var styles string

   // Styles is the shared CSS of every view: the palette in light and dark
   // mode, the common classes and the health classes. A page that shows any
   // view includes it once, before the CSS of the views.
   var Styles = template.CSS(styles)
   ```

8. In `view/raw/raw.html`, change `<div class="view-raw">` to `<div class="view view-raw">`.
9. Create `view/raw/doc.go` from the "Raw view" part of today's `view/doc.go`. Start it with `// Package raw is the raw view: it presents the signals of a source.Source as they were collected, before any model is built from them.` Keep the parts about the smaller views, the layout, data-scroll and the ok/bad/none marks. Add: "A page includes view.Styles and Styles."
10. Rewrite `view/doc.go` with the text under "Package documentation".
11. Workbench:
    - In `harness/workbench/workbench.go`, import `github.com/miroslav-matejovsky/inspector/view/raw`.
    - Replace `view.Raw(summaries)` with `raw.Render(summaries)`.
    - Change the page data field `Styles template.CSS` to `Styles []template.CSS` and set it to `[]template.CSS{view.Styles, raw.Styles}`.
    - In `harness/workbench/index.html`, replace `<style>{{.Styles}}</style>` with `{{range .Styles}}<style>{{.}}</style>{{end}}`.
12. In `harness/workbench/workbench_test.go`, `TestPageShowsRawView` uses `raw.Render` and requires that the body contains both `string(view.Styles)` and `string(raw.Styles)`.
13. In `.go-arch-lint.yml`:
    - Under `components`, keep `view: { in: view }` and add `view-raw: { in: view/raw }`.
    - Under `deps`, change `view` to have no dependencies (remove its `mayDependOn`) and add `view-raw: { mayDependOn: [source] }`.
    - Set `workbench.mayDependOn` to `[inspected, inspected-fulfillment, source, view, view-raw]`.
14. Documentation:
    - In `harness/workbench/doc.go`, change "the raw view of package view" to "the raw view of package view/raw" and "view.Raw ... styled by view.Styles" to "raw.Render ... styled by view.Styles and raw.Styles".
    - In `harness/README.md`, change "the raw view of package `view`" to "the raw view of package `view/raw`".
    - In the root `README.md`, replace the `view` row with the two rows under "README rows", and change "the raw view of package `view`" in the Harness paragraph to "the raw view of package `view/raw`".
15. Run `task all`.

## Technical Details

### CSS split

The new `view/view.css` holds everything shared, scoped to the class `view`:

```css
/* Shared palette and classes. The root element of every view has the class "view". */
.view {
  --view-border: #d0d0d7;
  --view-muted: #6b6b76;
  --view-code-bg: #f7f7f9;
  --view-ok: #1a7f37;
  --view-degraded: #9a6700;
  --view-bad: #cf222e;
  --view-unknown: #6b6b76;
  font-size: 13px;
}
@media (prefers-color-scheme: dark) {
  .view {
    --view-border: #34343d;
    --view-muted: #8d8d99;
    --view-code-bg: #18181d;
    --view-ok: #3fb950;
    --view-degraded: #d29922;
    --view-bad: #f85149;
    --view-unknown: #8d8d99;
  }
}
.view a { color: inherit; }
.view .num { text-align: right; font-variant-numeric: tabular-nums; }
.view .error { color: var(--view-bad); }
.view .note { margin: 4px 0; color: var(--view-muted); font-style: italic; }

/* Health of a model entity: --view-health is its color, text in that color,
   and a filled badge with the class "badge". */
.view .health-ok { --view-health: var(--view-ok); }
.view .health-degraded { --view-health: var(--view-degraded); }
.view .health-down { --view-health: var(--view-bad); }
.view .health-unknown { --view-health: var(--view-unknown); }
.view .health { color: var(--view-health); font-weight: 600; }
.view .badge {
  display: inline-block;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--view-health);
  color: #ffffff;
  font-size: 11px;
  font-weight: 600;
}
```

`view/raw/raw.css` is today's `view/view.css` with these changes:

- The first `.view-raw { ... }` block and its dark-mode block keep only the highlight variables `--view-key`, `--view-str`, `--view-num`, `--view-lit`, `--view-punct`, `--view-comment`, `--view-name` and `--view-label`. `--view-border`, `--view-muted`, `--view-code-bg`, `--view-ok` and `--view-bad` come from `.view`.
- These rules are removed because `.view` covers them: `.view-raw { font-size: 13px; }`, `.view-raw a`, `.view-raw .num`, `.view-raw .error` and `.view-raw .note`.
- Every other rule stays as it is. `.view-raw pre.body .num` still wins over `.view .num`: its specificity is (0,2,1) against (0,2,0).

### Package documentation

`view/doc.go`:

```go
// Package view is the Explain part of the Inspector toolkit: it presents what
// the toolkit knows about an inspected system in a form that humans read.
//
// Each kind of view is a sub-package:
//
//   - view/raw: the signals of a source.Source as they were collected.
//
// Views render HTML fragments with html/template, so every value taken from
// the inspected system is escaped. The root element of every view has the
// class "view" and a class of its own, "view-<name>"; the CSS of a view is
// scoped to its class and follows the light or dark color scheme of the
// browser.
//
// This package holds the CSS that every view shares, Styles: the palette,
// the classes num, note and error, and the health classes health-ok,
// health-degraded, health-down and health-unknown, which set --view-health
// for the classes health (colored text) and badge (filled label). A page
// includes Styles once, then the Styles of every view it shows.
package view
```

Steps 04, 05 and 06 add their sub-package to the list.

### README rows

```markdown
| `view` | Explain: shared CSS of every view (palette, health classes). | stdlib |
| `view/raw` | Raw view: collected signals with bodies formatted by content type. | `source` |
```

## Verification

Commands:

```powershell
go test ./view/... ./harness/workbench/... -v
go-arch-lint check
task all
```

| Test | Asserts |
| --- | --- |
| every test moved to `view/raw/raw_test.go` | passes unchanged, apart from `view.Raw` -> `raw.Render` and `view.Styles` -> `raw.Styles` |
| `TestRenderMarksViewRoot` (new, `view/raw`) | the output of `raw.Render` starts with `<div class="view view-raw">` |
| `TestStylesDefineSharedPalette` (new, `view`) | `view.Styles` contains `.view {`, `--view-ok`, `--view-degraded`, `--view-bad`, `--view-unknown` and `prefers-color-scheme: dark` |
| `TestStylesDefineHealthClasses` (new, `view`) | `view.Styles` contains `.health-ok`, `.health-degraded`, `.health-down`, `.health-unknown`, `.view .health {` and `.view .badge` |
| `TestStylesLeaveSharedRulesToView` (new, `view/raw`) | `raw.Styles` does not contain `--view-ok:` or `.view-raw .note` |
| `TestPageShowsRawView` (workbench, changed) | the page contains the output of `raw.Render`, `view.Styles` and `raw.Styles` |

## Acceptance Criteria

- `view/` contains only `doc.go`, `view.go`, `view.css`, `view_test.go` and the directory `raw/`.
- `go doc ./view` lists only `var Styles`. `go doc ./view/raw` lists `func Render` and `var Styles`.
- Every test in the table passes, and so do all moved raw tests.
- `git grep -n "view\.Raw" -- "*.go" ":!vendor"` finds nothing.
- `.go-arch-lint.yml` has components `view` and `view-raw`, `view-raw` may depend on `source` only, and `go-arch-lint check` passes.
- `task all` passes.

## Non-Goals

- No change to what the raw view shows or how it is laid out.
- No new view. Steps 04 to 06 add them.
- No change to the refresh script of the workbench.
