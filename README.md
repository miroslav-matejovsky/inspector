# Inspector

## Library

Inspector is a Go library. Each supporting domain of the [domain map](dev/principles/4-domain-map.md) has one top-level folder: `observation/`, `explanation/`, `navigation/`, `representation/`, `connectivity/`.

The library knows no inspected system:

- Library packages never import `harness/...`. `go-arch-lint` enforces it: library components list only library components in `mayDependOn`.
- Library folders never name the harness or its simulated domain, in code, comments, tests or docs. `task boundary` enforces it and runs in `task all`.

| Package | Domain | Responsibility | May depend on |
| --- | --- | --- | --- |
| `observation` | Observation | Domain-neutral model of observed state: entities, relations, snapshots. | stdlib |
| `navigation` | Navigation | Links of an entity in a snapshot, outgoing and incoming. | `observation` |
| `connectivity` | Connectivity | Read-only access to sources: GET-only JSON document reader over HTTP. | stdlib |
| `representation` | Representation | JSON views of snapshots over HTTP with navigable hrefs. | `observation`, `navigation` |

A consumer maps its own system into the library model. The harness maps the inspected simulation in `harness/adapter`.

## Harness

`harness/` holds the development workbench: a web page with an inspected panel and an inspector panel, plus a simulated order fulfillment service that exposes a JSON API, health checks and Prometheus metrics for Inspector to work against. Start it with `task workbench`. The workbench also serves the Inspector views of the simulation under `/inspector/`. See [harness/README.md](harness/README.md).
