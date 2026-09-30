# Inspector

Inspector is a Go library for inspecting running systems. Its principles are in [dev/principles.md](dev/principles.md).

## Library

The toolkit follows the Alignment principle: Observe -> Source, Navigate -> Model, Explain -> View. Top-level packages are the public toolkit; `internal/` holds their implementation details. Toolkit packages never import `harness/...`; `go-arch-lint` enforces it.

| Package | Responsibility | May depend on |
| --- | --- | --- |
| `source` | Observe: collects signals from HTTP targets on an interval and stores them. | `internal/signalstore` |
| `model` | Navigate: builds a read-only graph of entities with state, properties and links from the latest signals of a Source, through interpreters. | `source` |
| `internal/signalstore` | SQLite storage of collected signals. | stdlib, `github.com/ncruces/go-sqlite3` |
| `internal/logfile` | One log file per run, `log/slog` text format. | stdlib |
| `view` | Explain: shared CSS of every view (palette, health classes). | stdlib |
| `view/raw` | Raw view: collected signals with bodies formatted by content type. | `source` |
| `view/explorer` | Explain: entities of a model by kind; entity pages with state, properties, evidence, links and backlinks. | `model` |
| `view/chart` | Explain: SVG charts of a model: entity health by kind. | `model` |
| `view/dashboard` | Explain: overview of a model: health chart, degraded and down entities, model issues. | `model`, `view/chart` |

## Harness

`harness/` holds the development workbench: a web page with a single workbench panel, a simulated order fulfillment service that exposes a JSON API, health checks and Prometheus metrics for Inspector to work against. The workbench runs a `source.Source` against the simulation and shows a model of the inspected system built from the collected signals (dashboard, entity explorer) and the collected signals in the raw view. Start it with `task workbench`. Each run writes its log to `logs/`. See [harness/README.md](harness/README.md).
