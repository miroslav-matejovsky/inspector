# Architecture

The toolkit follows the Alignment principle of [dev/principles.md](../dev/principles.md): understanding starts with observation, grows by following relationships and ends in explanation. Each step is a group of packages:

```text
Observe  -> Source   package source
Navigate -> Model    package model
Explain  -> View     package view and its sub-packages
```

Data flows in one direction: a Source collects signals, a Model is built from the latest signals, and Views render a Model (or, for the raw view, the signals themselves) as HTML.

## Packages

Top-level packages are the public toolkit; `internal/` holds their implementation details.

| Package | Responsibility | May depend on |
| --- | --- | --- |
| `source` | Observe: collects signals from HTTP targets on an interval and stores them. | `internal/signalstore` |
| `model` | Navigate: builds a read-only graph of entities with state, properties and links from the latest signals of a Source, through interpreters. | `source` |
| `view` | Explain: shared CSS of every view (palette, health classes). | stdlib |
| `view/raw` | Raw view: collected signals with bodies formatted by content type. | `source` |
| `view/explorer` | Explain: entities of a model by kind; entity pages with state, properties, evidence, links and backlinks. | `model` |
| `view/chart` | Explain: SVG charts of a model: entity health by kind. | `model` |
| `view/dashboard` | Explain: overview of a model: health chart, degraded and down entities, model issues. | `model`, `view/chart` |
| `internal/signalstore` | SQLite storage of collected signals. | stdlib, `github.com/ncruces/go-sqlite3` |
| `internal/logfile` | One log file per run, `log/slog` text format. | stdlib |

## Rules

- Toolkit packages never import `harness/...`.
- A package imports only the packages listed in its row above.
- Kinds of entities, link types and property names are defined by interpreters, not by `model` or the views. A new kind of signal needs a new `model.Interpreter`; a new visualization is a new sub-package of `view`.

`go-arch-lint` enforces the dependency rules; the components and their allowed dependencies are in [.go-arch-lint.yml](../.go-arch-lint.yml).

## Harness

`harness/` is not part of the toolkit. It holds the development workbench (`harness/workbench`), which composes the toolkit against a simulated order fulfillment service (`harness/inspected`). See [harness/README.md](../harness/README.md).
