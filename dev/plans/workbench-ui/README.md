# Workbench UI: Inspector pages, dashboards and operator panel

## Goal

Make the workbench show the inspected system and what Inspector understands about it.

- The **inspector** panel shows HTML pages rendered by the library package `representation`: overview, entities, entity, explanation and dashboards. The pages are built by the same view builders as the existing JSON views, so both formats show the same facts and links.
- A **dashboard** is a model summary: an ordered list of panels over the observed entity model (entity counts by state, entity lists, pinned entities with reason and root causes). Dashboards are created and edited in the UI and kept by the library in one JSON file, so they survive restarts.
- The **inspected** panel shows an operator panel owned by the harness. It drives the simulation (clock, advance, dependency modes, place order) so that a user can create situations and watch Inspector observe and explain them.

## Scope

In scope:

- Package `representation/dashboard`: dashboard definitions, validation and `FileStore`, a JSON file store with atomic writes.
- Package `representation`: view builders shared by JSON and HTML, entity list filter `state`, HTML pages under `{prefix}/ui/`, dashboard JSON views, dashboard HTML pages, dashboard editing through HTML forms, cross-origin protection of form posts.
- Package `harness/workbench`: operator panel under `/operator/`, two new required flags, the dashboard store opened by `Run`, both panels embedding their pages in iframes.
- `Taskfile.yml`, `.gitignore`, `.go-arch-lint.yml`, `README.md`, `harness/README.md`, package docs and `.todo` live verification.

Out of scope:

- Metrics (`/inspected/metrics`) and charts.
- Snapshot history. Every view observes the source again, as today.
- JavaScript. Pages are rendered on the server; interaction is links and HTML forms.
- A JSON write API for dashboards. Dashboards are written only through the UI forms.
- Renaming dashboards and reordering panels.
- Authentication. One process owns the dashboards file.
- Any change to `observation`, `navigation`, `explanation`, `connectivity`, `harness/adapter` and `harness/inspected`.

## Boundary

```text
cmd/workbench -> harness/workbench -+-> harness/inspected                        serves /inspected/...
                                    +-> operator (in harness/workbench)          serves /operator/...
                                    |      '-- HTTP GET, PUT, POST --> /inspected/sim/..., /inspected/api/...
                                    +-> harness/adapter -> connectivity          HTTP GET only --> /inspected/...
                                    +-> representation -+-> representation/dashboard
                                    |                   +-> explanation, navigation, observation
                                    +-> representation/dashboard                 opens the dashboards file
```

| Rule | Enforcement | Step |
| --- | --- | --- |
| Inspector never writes to the inspected system. Its only write path is its own dashboards file. | `connectivity` stays GET-only (unchanged). UI form handlers call only the dashboard store. `TestUIWritesDoNotObserve` proves form posts do not call the observer. | 05 |
| The operator panel is harness, not Inspector. It has its own HTTP client and never uses `connectivity`. | Lives in `harness/workbench`; go-arch-lint keeps library packages from importing it. | 06 |
| Library pages and dashboards never name the harness or its simulated domain. | `task boundary` scans `representation` recursively, including templates and `representation/dashboard`. | 01 to 05 |
| Library tests use neutral fixtures: `book`, `author`, `member`, `room`, `heater`, `fuse`, `lamp`. | `task boundary` scans test files. | 01 to 05 |
| Kinds, IDs and states in dashboard panels are opaque strings chosen by the user. | Library code never compares them to literals. | 01, 04 |
| Templates read only fields, never call methods of types of this module. | Code review of the templates; `task deadcode` cannot see calls made by reflection. | 03 to 06 |

## Architecture impact

New packages:

| Package | Domain | Responsibility | May depend on |
| --- | --- | --- | --- |
| `representation/dashboard` | Representation | Dashboard definitions, validation, JSON file store. | stdlib |

Changed:

| Artifact | Change | Step |
| --- | --- | --- |
| `representation` | View builders with a link base; `state` filter; HTML pages under `{prefix}/ui/`; dashboard views; form posts; `NewHandler` takes a `Dashboards` store. | 02 to 05 |
| `harness/workbench` | `InspectorHandler` takes the store; `OperatorHandler`; `Handler` mounts `/operator/`; `Config.InspectorDashboardsFile`, `Config.OperatorTimeout`; `index.html` embeds both UIs. | 04, 06, 07 |
| `cmd/workbench/main.go` | Prints the operator and inspector UI URLs. | 06, 07 |
| `.go-arch-lint.yml` | Component `dashboard`; `representation` and `workbench` may depend on it. | 01, 04 |
| `Taskfile.yml` | `workbench` passes `-inspector-dashboards-file` and `-operator-timeout`. | 04, 06 |
| `.gitignore` | `.workbench-dashboards.json` and its temporary file. | 04 |
| `README.md`, `harness/README.md`, `representation/doc.go`, `harness/workbench/doc.go`, `.todo` | Documentation of the new packages, endpoints, flags and live verification. | 01 to 07 |

No new third-party module. `go.mod`, `go.sum` and `vendor/` do not change.

New endpoints of the library, relative to the prefix (`/inspector` in the workbench):

| Method | Path | Purpose | Step |
| --- | --- | --- | --- |
| GET | `/entities?kind=&state=` | Existing list, new optional filter `state` | 02 |
| GET | `/ui/` | Overview page | 03 |
| GET | `/ui/entities` | Entity list page, filters `kind` and `state` | 03 |
| GET | `/ui/entities/{kind}/{id}` | Entity page | 03 |
| GET | `/ui/entities/{kind}/{id}/explanation` | Explanation page | 03 |
| GET | `/dashboards` | Dashboard list (JSON) | 04 |
| GET | `/dashboards/{id}` | Evaluated dashboard (JSON) | 04 |
| GET | `/ui/dashboards` | Dashboard list page | 04 |
| GET | `/ui/dashboards/{id}` | Dashboard page | 04 |
| POST | `/ui/dashboards` | Create a dashboard | 05 |
| POST | `/ui/dashboards/{id}/delete` | Delete a dashboard | 05 |
| POST | `/ui/panels` | Add a panel to a dashboard | 05 |
| POST | `/ui/dashboards/{id}/panels/{index}/delete` | Remove a panel | 05 |

Every `/ui/` GET page accepts `?refresh=<seconds>`, 1 to 3600, which reloads the page on that interval.

New workbench endpoints:

| Method | Path | Purpose | Step |
| --- | --- | --- | --- |
| GET | `/operator/` | Operator page: simulation status, dependencies, products | 06 |
| POST | `/operator/clock` | Pause or resume the clock | 06 |
| POST | `/operator/advance` | Advance N ticks | 06 |
| POST | `/operator/dependencies/{name}` | Set a dependency mode | 06 |
| POST | `/operator/orders` | Place an order | 06 |

## Principles evaluation

| Principle | How this plan supports it |
| --- | --- |
| [Observe](../../principles/2-observe.md) | HTML pages show the same facts as the JSON views: state, attributes, history, relations, observation time and gaps, on every page. No caching: every page observes again, and opt-in auto-refresh keeps a page current. Inspector gains a write path only to its own dashboards file; `connectivity` stays GET-only and form posts never call the observer. Controlling the simulation is the operator panel, which is harness code with its own client, shown in the separate inspected panel. |
| [Explain](../../principles/1-explain.md) | The explanation page renders the cause tree with reasons and history and lists the root causes. The entity panel of a dashboard shows state, reason and root causes of a pinned entity and links to its explanation. Every entity page links to "why is it in this state". |
| [Navigate](../../principles/3-navigate.md) | HTML hrefs come from the same view builders as JSON hrefs, with a different base. Every kind, entity, related entity, cause, root cause and state count is a link. Dashboards are built while navigating: overview, entity list and entity pages each offer "add to dashboard". |
| [Domain Map](../../principles/4-domain-map.md) | Pages and dashboards are Representation: they present observations, explanations and relations. Dashboard definitions are Representation settings, so they live in `representation/dashboard`. Observation, Explanation, Navigation and Connectivity do not change. The library stays free of the inspected domain; `task boundary` covers templates and the new package. |

## Deliverables

- Package `representation/dashboard` with `doc.go` and tests.
- `representation`: `views.go`, `html.go`, `forms.go`, `templates/*.html`, tests `html_test.go`, `dashboard_view_test.go`, `forms_test.go`; updated `handler.go`, `resources.go`, `doc.go`, `handler_test.go`.
- `harness/workbench`: `operator.go`, `operator.html`, `operator_test.go`; updated `workbench.go`, `config.go`, `index.html`, `doc.go` and tests.
- Updated `cmd/workbench/main.go`, `Taskfile.yml`, `.gitignore`, `.go-arch-lint.yml`, `README.md`, `harness/README.md`, `.todo`.

## Success criteria

- `task all` exits 0; its output contains `boundary: no issues found` and `deadcode: no issues found`.
- Every test named in steps 01 to 07 exists and passes.
- `go list -deps ./representation/...` prints no package path containing `/harness/`.
- `git diff --stat` shows no change in `observation/`, `navigation/`, `explanation/`, `connectivity/`, `harness/adapter/`, `harness/inspected/`, `go.mod`, `go.sum`, `vendor/`.
- `TestOperatorDrivesInspectorDashboard` (step 07) passes: an outage set through `/operator/` is visible in a dashboard created through `/inspector/ui/` in the same handler.
- `.todo` contains the section `## Workbench UI`.

## Steps

| Step | Title | Depends on |
| --- | --- | --- |
| [01](01-dashboard-model-and-store.md) | Dashboard model and file store | - |
| [02](02-view-builders-and-state-filter.md) | View builders and state filter | - |
| [03](03-html-pages.md) | HTML pages | 02 |
| [04](04-dashboard-views.md) | Dashboard views and store wiring | 01, 03 |
| [05](05-dashboard-editing.md) | Dashboard editing | 04 |
| [06](06-operator-panel.md) | Operator panel | 04 |
| [07](07-workbench-page.md) | Workbench page and documentation | 05, 06 |

Supporting documents: [alternatives.md](alternatives.md), [assessment.md](assessment.md), [progress.md](progress.md).
