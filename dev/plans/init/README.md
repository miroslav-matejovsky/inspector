# Init: Inspector library

## Goal

Start the Inspector library. Build the first working slice through four of the five supporting domains: a domain-neutral model of observed state (Observation), links between observed entities (Navigation), read-only access to a source (Connectivity) and JSON views (Representation). The workbench consumes the library against the inspected simulation and serves the views under `/inspector/`.

The simulated fulfillment domain stays in the harness. The library never imports the harness and never names the simulated domain. Both rules are enforced by `task all`.

## Scope

In scope:

- Boundary guard: `task boundary` rejects harness and fulfillment vocabulary in the library folders; go-arch-lint rejects harness imports from library packages.
- Package `observation`: `Ref`, `Entity`, `Attribute`, `Relation`, `Snapshot`, validated by `NewSnapshot`.
- Package `navigation`: `Links` of one entity in a snapshot, outgoing and incoming.
- Package `connectivity`: `HTTPReader`, a GET-only JSON document reader.
- Package `representation`: JSON views of snapshots over HTTP (overview, entity list, entity detail with related entities).
- Package `harness/adapter`: maps the read-only inspected HTTP API to an `observation.Snapshot`.
- Workbench wiring: `/inspector/` mounted next to `/inspected/`, one new required flag, `task workbench`, documentation, `.todo` live verification.

Out of scope:

- Package `explanation`. The `explanation/` folder keeps its `README.md` and gets no Go code.
- HTML views. `harness/workbench/index.html` is not changed; both panels stay empty.
- Caching, polling, or history of snapshots. Every view request observes the source again.
- Order history (`history` of the orders API), metrics (`/inspected/metrics`) and simulation control (`/inspected/sim`). The adapter reads only `/inspected/health/ready`, `/inspected/api/products` and `/inspected/api/orders`.
- Sources other than HTTP with JSON bodies.

## Boundary

This is the central design rule of the plan.

```text
cmd/workbench -> harness/workbench -+-> harness/inspected               (serves /inspected/...)
                                    +-> harness/adapter --+-> observation
                                    |                     +-> connectivity
                                    +-> representation ---+-> navigation -> observation
                                    |                     +-> observation
                                    +-> connectivity

harness/adapter reads harness/inspected only over HTTP GET, never by Go import.
Library packages: observation, navigation, connectivity, representation (and later explanation).
Library packages import nothing from harness/.
```

| Rule | Enforcement | Step |
| --- | --- | --- |
| Library packages never import `harness/...`. | go-arch-lint: each library component lists only library components in `mayDependOn`. A package that is not mapped to a component fails `go-arch-lint check` (verified by probe, see [assessment.md](assessment.md)). | 01 to 06 |
| Library folders never name the harness or the simulated domain, in code, comments, tests or docs. | `task boundary` (`taskfile/boundary.ps1`), part of `task all`. | 01 |
| Library tests use a neutral fixture vocabulary: `book`, `author`, `written_by`, `cites`. | `task boundary` scans test files too. | 02 to 06 |
| All knowledge of the inspected JSON contract and all mapping to `observation` lives in `harness/adapter`. | The adapter is the only package that depends on `observation` and knows `/inspected/...` paths. | 05 |
| `harness/adapter` does not import `harness/inspected` in production code. | go-arch-lint: `adapter: { mayDependOn: [observation, connectivity] }`. Test files are excluded from arch-lint, so a contract test may import `harness/inspected`. | 05 |
| Kinds, IDs, states, attribute names and relation kinds are opaque strings chosen by the source. | Library code never compares them to literals; `task boundary` rejects the known literals. | 02 to 06 |

## Architecture impact

New packages:

| Package | Domain | Responsibility | May depend on |
| --- | --- | --- | --- |
| `observation` | Observation | Domain-neutral model of observed state. Pure, no I/O. | stdlib |
| `navigation` | Navigation | Links of an entity in a snapshot. Pure, no I/O. | `observation` |
| `connectivity` | Connectivity | GET-only JSON document reader over HTTP. | stdlib |
| `representation` | Representation | JSON views of snapshots over HTTP. | `observation`, `navigation` |
| `harness/adapter` | (consumer) | Inspected HTTP API to `observation.Snapshot`. | `observation`, `connectivity` |

Changed:

| Artifact | Change | Step |
| --- | --- | --- |
| `taskfile/boundary.ps1` | New vocabulary check. | 01 |
| `Taskfile.yml` | Task `boundary`, run by `all`; `workbench` passes `-inspector-source-timeout`. | 01, 07 |
| `.go-arch-lint.yml` | Library rule comment, components `observation`, `navigation`, `connectivity`, `representation`, `adapter`, updated `workbench` rule. | 01 to 07 |
| `observation/README.md`, `navigation/README.md`, `connectivity/README.md`, `representation/README.md` | Replaced by `doc.go` (Go packages have no `README.md`). | 02, 03, 04, 06 |
| `harness/workbench` | `InspectorPathPrefix`, `InspectorHandler`, `Handler` mounts the inspector, `Config.InspectorSourceTimeout`, flag `-inspector-source-timeout`. | 07 |
| `cmd/workbench/main.go` | Prints the inspector URL. | 07 |
| `README.md`, `harness/README.md`, `.todo` | Library section, adapter section, live verification. | 01 to 07 |

No new third-party module. `go.mod` and `go.sum` do not change.

Inspector endpoints (served by the workbench, step 06 and 07):

| Method | Path | Purpose | Status codes |
| --- | --- | --- | --- |
| GET | `/inspector/` | Overview: observation time, entity kinds with counts | 200, 503 |
| GET | `/inspector/entities` | Entities, optional filter `kind` | 200, 503 |
| GET | `/inspector/entities/{kind}/{id}` | One entity with attributes and related entities | 200, 404, 503 |

## Principles evaluation

| Principle | How this plan supports it |
| --- | --- |
| [Observe](../../principles/2-observe.md) | `observation.Snapshot` is a faithful, validated picture of what the source reports: entities, states, attributes, relations, observation time. `connectivity.HTTPReader` can only send GET requests, so Inspector cannot modify the inspected system. The adapter reads only observation endpoints, never `/inspected/sim`. Every view request observes again, so views show current state. |
| [Navigate](../../principles/3-navigate.md) | `navigation.Links` answers "what is related" in both directions. Every view carries `href` links, so a client moves from the overview to a kind, to an entity, to its related entities without building URLs. |
| [Explain](../../principles/1-explain.md) | This plan implements no explanation logic. Causes that the source reports are kept as observed facts and are visible: the service state and each health check state with its `reason`, and each order state with its `failure_reason`. |
| [Domain Map](../../principles/4-domain-map.md) | One top-level package per supporting domain, named after it. Dependencies follow the map: Representation uses Observation and Navigation, Navigation uses Observation, Connectivity is independent of the model. The inspected system is only reached through Connectivity and a consumer-side adapter. |

## Deliverables

- Packages `observation`, `navigation`, `connectivity`, `representation`, `harness/adapter`, each with `doc.go` and tests.
- `taskfile/boundary.ps1` and task `boundary` in `task all`.
- Updated `harness/workbench`, `cmd/workbench/main.go`, `Taskfile.yml`, `.go-arch-lint.yml`.
- Updated `README.md`, `harness/README.md`, `.todo`.

## Success criteria

- `task all` exits with code 0 and its output contains `boundary: no issues found`.
- Every test named in steps 02 to 07 exists and passes.
- `go list -deps ./observation ./navigation ./connectivity ./representation` prints no package path containing `/harness/`.
- `go list ./observation ./navigation ./connectivity ./representation ./harness/adapter` lists 5 packages.
- `explanation/` contains only `README.md`.
- `git diff --stat` shows no change to `harness/inspected/`, `harness/workbench/index.html`, `go.mod`, `go.sum`.
- Test `TestInspectorHandlerObservesInspected` (step 07) gets HTTP 200 from `/inspector/` through `workbench.Handler` with kinds `service`, `health_check` and `product`.
- `.todo` contains the section `## Inspector in the workbench`.

## Steps

| Step | Title | Depends on |
| --- | --- | --- |
| [01](01-library-boundary.md) | Library boundary guard | - |
| [02](02-observation-model.md) | Observation model | 01 |
| [03](03-navigation-links.md) | Navigation links | 02 |
| [04](04-connectivity-http-reader.md) | Connectivity HTTP reader | 01 |
| [05](05-harness-adapter.md) | Harness adapter | 02, 04 |
| [06](06-representation-json-views.md) | Representation JSON views | 02, 03 |
| [07](07-workbench-integration.md) | Workbench integration and documentation | 05, 06 |

Supporting documents: [alternatives.md](alternatives.md), [assessment.md](assessment.md), [progress.md](progress.md).
