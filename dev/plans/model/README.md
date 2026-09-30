# Model

## Goal

Add the Navigate part of the toolkit. A new top-level package `model` turns the latest signals of a `source.Source` into a read-only graph of entities with state, properties and links. The workbench defines its model of the inspected system with it. New views in sub-packages of `view` show that model. The existing raw view moves to `view/raw`.

After the plan, the workbench has three pages:

| Page | Path | Shows |
| --- | --- | --- |
| Dashboard | `/` | health of the entities by kind (SVG chart), the entities that are degraded or down, and the model issues |
| Model | `/model` | the entities grouped by kind; `/model?entity=<id>` shows one entity with its links and backlinks |
| Raw | `/raw` | the raw view of today, unchanged in content |

## Scope

In scope:

- Package `model`: the types `Entity`, `State`, `Health`, `Property`, `Link`, `Backlink`, `Evidence` and `Issue`; the types `Interpreter` and `Model`; the functions `Build` and `New`; and the read methods `Entities`, `Entity`, `Backlinks` and `Issues`.
- The inspected model of the workbench: interpreters for five observation endpoints of inspected (`/inspected/`, `/inspected/health/ready`, `/inspected/api/dependencies`, `/inspected/api/products`, `/inspected/api/orders`). They define six kinds (service, readiness, check, dependency, product, order), six link types and the health mapping.
- The `view` package is split. The root package `view` holds the shared palette and health classes. `view/raw` is the moved raw view. `view/explorer` shows the entity index and the entity pages. `view/chart` draws the health bars. `view/dashboard` composes the chart and the attention list.
- Workbench pages `/`, `/model` and `/raw` with tabs. The dependency controls return to the page they were used on.
- Documentation: the root `README.md`, `harness/README.md`, every `doc.go` involved, `.go-arch-lint.yml`, and `.todo` for live verification.

Out of scope:

- History and time series: the model is built from `source.TargetSummary.Latest` only.
- Interpreting `/inspected/metrics` and `/inspected/health/live`. They stay visible in the raw view.
- Caching or persisting models: a model is built for every page render.
- Changes to package `source` or package `inspected`.
- Changes to the dependency controls beyond the return path. They keep reading `GET /inspected/sim`, because they are the control plane, not observation.

## Principles

This plan was evaluated against `dev/principles.md`.

| Principle | How the plan follows it |
| --- | --- |
| Observe | The model is built only from signals that the Source collected with GET. The workbench interpreters read the documented JSON contract of inspected; they do not import the inspected domain packages. Entity IDs are the self links that inspected states. Every entity carries `Evidence`: the target, URL and time of the signal it was read from. |
| Navigate | The model is entities, links, properties and state. Backlinks are computed, so every entity shows what points to it. The explorer view turns every link and backlink into a link to the page of the other entity. A link to an entity that no signal described is kept and shown as "not observed", not hidden. |
| Explain | The dashboard shows health by kind and lists the degraded and down entities with the reason the system states. Links from there lead to the cause: for example check -> "determined by" -> dependency, and order -> "decided by" -> dependency. Model issues show what could not be understood. |
| Proximity | Everything runs in the workbench process on the same page. There is no new storage, service or dependency. |
| Alignment | Source -> Model -> View is the package structure: `model` may depend only on `source`, model views may depend only on `model`, and `go-arch-lint` enforces it. The workbench composes them. |

## Architecture Impact

Packages after the plan:

| Package | Part | Responsibility | May depend on |
| --- | --- | --- | --- |
| `source` | Observe | unchanged | `internal/signalstore` |
| `model` | Navigate | builds a read-only entity graph from the latest signals through interpreters | `source` |
| `view` | Explain | shared palette and health classes of every view (CSS only) | stdlib |
| `view/raw` | Explain | collected signals with bodies formatted by content type (moved from `view`) | `source` |
| `view/explorer` | Explain | entity index by kind; entity page with state, properties, evidence, links and backlinks | `model` |
| `view/chart` | Explain | SVG chart of entity health by kind | `model` |
| `view/dashboard` | Explain | health chart, entities that need attention, model issues | `model`, `view/chart` |
| `harness/workbench` | Harness | inspected model (interpreters) and pages `/`, `/model`, `/raw` | `inspected`, `inspected-fulfillment`, `source`, `model`, `view`, `view-raw`, `view-explorer`, `view-chart`, `view-dashboard` |

Dependency direction:

```text
source <- model <- view/explorer
                <- view/chart <- view/dashboard
source <- view/raw
harness/workbench -> model, view, view/raw, view/explorer, view/chart, view/dashboard
```

### Extensibility

The model can grow without breaking what exists. These rules are written into `model/doc.go` in step 01:

- Kinds, link types and property names are data that the interpreters define. Package `model` validates only the structure and never branches on these values. A new kind of entity or relation needs no change in `model` or in any view.
- A new signal needs a new `Interpreter` and nothing else. The caller selects interpreters per target.
- A new field on `Entity`, `State`, `Property`, `Link`, `Evidence` or `Issue` has a zero value that means "not stated", so every existing interpreter and view keeps working. Values are built with keyed composite literals; `go vet` (composites) enforces this outside the package.
- A new query is a new method on `*Model`; existing methods keep their contract.
- `Health` is the only closed set, because views color by it. Adding a value is a deliberate change of every view.
- A new visualization is a new sub-package of `view` that reads `*model.Model` through its methods. Existing views do not change.

## Deliverables

- `model/` with `doc.go`, `model.go`, `build.go` and tests.
- `harness/workbench/inspectedmodel.go`, `harness/workbench/inspectedmodel_test.go` and `harness/workbench/export_test.go`.
- `view/doc.go`, `view/view.go`, `view/view.css` and `view/view_test.go` (shared palette).
- `view/raw/` (moved raw view), `view/explorer/`, `view/chart/` and `view/dashboard/`, each with `doc.go`, code, templates, CSS and tests.
- Workbench pages and routes in `harness/workbench/workbench.go`, `harness/workbench/controls.go` and `harness/workbench/index.html`.
- Updated `.go-arch-lint.yml`, `taskfile/deadcode.ps1` (the allowlist is empty again at the end), `README.md`, `harness/README.md`, `harness/workbench/doc.go` and `.todo`.

## Success Criteria

- `task all` passes after every step.
- `go test ./model/... ./view/... ./harness/workbench/...` passes, including every test named in the steps.
- `go list -f '{{join .Imports " "}}' ./model` prints only standard library packages and `github.com/miroslav-matejovsky/inspector/source`.
- `go-arch-lint check` passes with the components and dependencies listed under Architecture Impact.
- `taskfile/deadcode.ps1` has an empty `$allow` list after step 07.
- `TestInspectedModelLinksResolve` passes: with all five interpreted targets collected from a running inspected app, every link in the model resolves to an entity and the model has no issues.
- `GET /`, `GET /model`, `GET /model?entity=/inspected/api/dependencies/payment-gateway` and `GET /raw` answer 200 in the workbench tests. `GET /model?entity=/nope` answers 404.

## Steps

| Step | Title | Dependencies | Effort | Complexity |
| --- | --- | --- | :---: | :---: |
| [01](01-model-package.md) | Model package | - | M | medium |
| [02](02-view-split.md) | Split view, move raw view | - | S | low |
| [03](03-workbench-inspected-model.md) | Inspected model in the workbench | 01 | M | medium |
| [04](04-view-explorer.md) | Explorer view | 01, 02 | M | medium |
| [05](05-view-chart.md) | Health chart view | 01, 02 | S | medium |
| [06](06-view-dashboard.md) | Dashboard view | 01, 02, 05 | S | low |
| [07](07-workbench-pages.md) | Workbench pages | 02, 03, 04, 05, 06 | L | medium |

Steps 01 and 02 can be done in either order. Steps 04 and 05 can be done in either order. Steps 01, 03, 04, 05 and 06 add functions that no command reaches yet, so they add them to the deadcode allowlist. Step 07 wires everything into the workbench and empties the allowlist.
