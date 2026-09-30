---
title: "04 - Explorer view"
dependencies: ["01", "02"]
effort: "M"
complexity: "medium"
---

# 04 - Explorer view

## Objective

The package `view/explorer` renders a `*model.Model` for navigation:

- `explorer.Index` shows every kind with a table of its entities.
- `explorer.Entity` shows one entity with its state, properties, evidence, links and backlinks.

Every related entity is a link to its own entity page. A link to an ID that is not in the model is shown as text marked "not observed". Nothing outside tests calls the package yet; its functions are staged in the deadcode allowlist until step 07.

## Target Artifacts

| File | Change |
| --- | --- |
| `view/explorer/doc.go` | new |
| `view/explorer/explorer.go` | new: `Index`, `Entity`, `Styles`, data building |
| `view/explorer/explorer.html` | new: templates `index`, `entity`, `ref` |
| `view/explorer/explorer.css` | new |
| `view/explorer/explorer_test.go` | new (package `explorer_test`) |
| `view/doc.go` | list `view/explorer` |
| `.go-arch-lint.yml` | component `view-explorer`, may depend on `model` |
| `taskfile/deadcode.ps1` | staged names |
| `README.md` | Library table: row `view/explorer` |

## Implementation Tasks

1. Write `view/explorer/explorer_test.go` with the tests under Verification.
2. Create `explorer.go`, `explorer.html` and `explorer.css` with the contracts under Technical Details, and make the tests pass.
3. Create `view/explorer/doc.go` with the text under "Package documentation".
4. In `view/doc.go`, add the list item `//   - view/explorer: the entities of a model.Model by kind, and one entity with its links and backlinks.`
5. In `.go-arch-lint.yml`, add the component `view-explorer: { in: view/explorer }` and the deps entry `view-explorer: { mayDependOn: [model] }`.
6. Add the row `` | `view/explorer` | Explain: entities of a model by kind; entity pages with state, properties, evidence, links and backlinks. | `model` | `` to the root `README.md` Library table, after `view/raw`.
7. Run `deadcode ./cmd/...` and stage the new names for `view/explorer/` in `taskfile/deadcode.ps1`, under the step 01 comment.
8. Run `task all`.

## Technical Details

### API

```go
package explorer

// maxRows is the number of entities shown per kind by Index and of
// backlinks shown by Entity.
const maxRows = 100

// Styles is the CSS of the explorer views. A page includes view.Styles and
// then Styles, each once.
var Styles template.CSS

// Index renders one section per kind, in the order in which the kinds first
// appear in m.Entities(). A section has the kind and its number of entities
// as heading and a table of its first maxRows entities: name (linked to the
// entity page), health badge, state value, reason, number of links and
// number of backlinks. The number of entities left out follows the table.
// A model without entities renders the note "no entities".
func Index(m *model.Model, entityPage string) (template.HTML, error)

// Entity renders the entity id of m: kind, name, health badge, state value
// and reason; ID and evidence (target, URL, observation time in RFC 3339);
// properties; links; and the first maxRows backlinks with the number of
// backlinks left out. Every related entity is shown with its kind, name and
// health and linked to its entity page; an ID that is not in m is shown as
// text with the note "not observed". Entity returns an error when id is not
// in m.
func Entity(m *model.Model, id, entityPage string) (template.HTML, error)
```

`entityPage` is the path of the page that shows `Entity` for the query parameter `entity`. The URL of an entity is `entityPage + "?entity=" + url.QueryEscape(id)`. The error of `Entity` for a missing ID is `explorer: entity "<id>" is not in the model`. A failed template execution returns `explorer: render index: <err>` or `explorer: render entity: <err>`.

### Template data

```go
// ref is a related entity in a view.
type ref struct {
	Href     string // entity URL; empty when the ID is not in the model
	ID       string
	Kind     string // empty when the ID is not in the model
	Name     string // the ID when not in the model
	Health   model.Health // model.HealthUnknown when not in the model
}

type kindSection struct {
	Kind   string
	Count  int
	Rows   []entityRow // at most maxRows
	Hidden int         // Count - len(Rows)
}

type entityRow struct {
	Ref       ref
	Value     string // State.Value, "-" when empty
	Reason    string // State.Reason, "-" when empty
	Links     int
	Backlinks int
}

type relation struct {
	Type string
	Ref  ref
}

type entityPageData struct {
	Entity     model.Entity
	Observed   string // Evidence.ObservedAt in RFC 3339, UTC
	Links      []relation
	Backlinks  []relation // at most maxRows
	Hidden     int        // backlinks left out
}
```

`newRef(m, id, entityPage)` fills a `ref` from `m.Entity(id)`. It is the only place that builds entity URLs.

### Markup

The root element of both views is `<div class="view view-explorer">`.

- Index: for each section, `<section class="kind"><h3>{{.Kind}} <span class="count">{{.Count}}</span></h3><table class="entities">` with the header cells `Name`, `Health`, `State`, `Reason`, `Links` and `Backlinks`. Each row is `<tr class="health-{{.Ref.Health}}">`. The name cell renders the template `ref`. The health cell is `<span class="badge">{{.Ref.Health}}</span>`, and the count cells use `class="num"`. After the table, `{{if .Hidden}}<p class="note">{{.Hidden}} more not shown</p>{{end}}`.
- Entity: `<section class="entity health-{{.Entity.State.Health}}">`, then:
  - the heading `<h3><span class="kind">{{kind}}</span> <span class="name">{{name}}</span> <span class="badge">{{health}}</span></h3>`;
  - `<p class="state">` with the value and the reason (each only when it is not empty);
  - `<dl class="meta">` with `ID`, and `Read from` as target, `<a href="{{URL}}">{{URL}}</a>` and the time;
  - the tables `Properties` (name, value), `Links` (type, entity) and `Backlinks` (type, entity), each with `<p class="note">none</p>` when empty, and the hidden count after the backlinks.
- Template `ref`: with `Href`, `<a href="{{.Href}}"><span class="kind">{{.Kind}}</span> {{.Name}}</a> <span class="health">{{.Health}}</span>` inside a `<span class="ref health-{{.Health}}">`. Without `Href`, `<span class="ref missing">{{.ID}} <span class="note">not observed</span></span>`.

### CSS

`explorer.css` is scoped to `.view-explorer`. It styles the tables `entities`, `properties` and `links` like the raw targets table (1px `--view-border` bottom lines, 4px 10px padding, 11px uppercase `--view-muted` headers). It adds a 16px bottom margin to `section.kind`, sets `.kind` labels in `--view-muted`, and sets `dl.meta` as a two-column grid (`grid-template-columns: max-content 1fr`). Colors come only from the variables of `view.Styles`.

### Package documentation

`view/explorer/doc.go`:

```go
// Package explorer renders a model.Model for navigation (Navigate -> Model,
// Explain -> View).
//
// Index lists the entities of every kind, in the order of the model; Entity
// shows one entity with its state, properties, the evidence it was read
// from, its links and its backlinks. Every related entity is a link to its
// entity page, entityPage?entity=<id>, which the page that embeds the view
// serves with Entity; a related ID that is not in the model is shown as
// "not observed". Index shows at most 100 entities per kind and Entity at
// most 100 backlinks, each followed by the number left out.
//
// A page includes view.Styles and Styles.
package explorer
```

## Verification

Commands:

```powershell
go test ./view/explorer/... -v
go-arch-lint check
task all
```

The tests build models with `model.New(entities, nil)`.

| Test | Asserts |
| --- | --- |
| `TestIndexGroupsEntitiesByKind` | entities of kinds `b`, `a`, `b` give sections `b` (count 2) then `a` (count 1) |
| `TestIndexLinksEntityPages` | entity `/x/1` gives `href="/model?entity=%2Fx%2F1"` with entityPage `/model` |
| `TestIndexMarksHealth` | a down entity row has `class="health-down"` and `<span class="badge">down</span>` |
| `TestIndexShowsDashForEmptyState` | an entity without value and reason has two `>-<` cells |
| `TestIndexCountsLinks` | an entity with 2 links and 1 backlink shows `>2<` and `>1<` in `num` cells |
| `TestIndexLimitsRows` | 101 entities of one kind give 100 rows and `<p class="note">1 more not shown</p>` |
| `TestIndexShowsEmptyModel` | `New(nil, nil)` renders `<p class="note">no entities</p>` |
| `TestIndexEscapesValues` | name `<script>x</script>` appears as `&lt;script&gt;x&lt;/script&gt;` and not as `<script>` |
| `TestEntityShowsState` | kind, name, badge, value `outage` and reason `calls fail` are present |
| `TestEntityShowsEvidence` | target `deps`, `<a href="http://example.test/deps">` and `2026-09-30T01:02:03Z` are present |
| `TestEntityShowsProperties` | each property name and value is present, in order |
| `TestEntityLinksRelatedEntities` | a link `waits on` to entity `/d` renders the type and `href="/model?entity=%2Fd"` with the kind, name and health of `/d` |
| `TestEntityMarksMissingEntities` | a link to `ghost` renders `ghost` and `not observed` and no `href` containing `ghost` |
| `TestEntityListsBacklinks` | entities `a` and `c` linking to `b` show both, in order, on the page of `b` |
| `TestEntityLimitsBacklinks` | 101 backlinks give 100 rows and `1 more not shown` |
| `TestEntityShowsNoneForEmptySections` | an entity without properties, links and backlinks shows `<p class="note">none</p>` three times |
| `TestEntityRejectsUnknownID` | `Entity(m, "nope", "/model")` returns an error that contains `entity "nope" is not in the model` |
| `TestViewsMarkRoot` | both outputs start with `<div class="view view-explorer">` |
| `TestStylesStyleExplorer` | `Styles` contains `.view-explorer` |

## Acceptance Criteria

- Every test in the table passes.
- `go list -f '{{join .Imports " "}}' ./view/explorer` prints only standard library packages and `github.com/miroslav-matejovsky/inspector/model`.
- `.go-arch-lint.yml` has `view-explorer` with `mayDependOn: [model]`, and `go-arch-lint check` passes.
- `view/doc.go` and the root `README.md` list `view/explorer`.
- `task all` passes.

## Non-Goals

- No filtering, sorting or search in the index: the order is the model order.
- No graph drawing of relations.
- No client-side script: navigation is plain links.
