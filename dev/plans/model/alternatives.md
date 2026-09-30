# Alternatives

Approaches that were considered for the model plan. Each has its trade-offs and the decision.

## Shape of the model

| Option | Trade-offs | Decision |
| --- | --- | --- |
| Generic entity graph: `Entity` with open `Kind`, `Properties`, `Links`, `State` | Domain-agnostic. A new kind or relation needs no change in `model` or the views. Values are text, so charts of numbers need a later typed value. | **Chosen.** It matches the Alignment principle: models organize information into entities, relationships, properties and state. |
| Typed Go structs per inspected resource in `model` (`model.Order`, `model.Product`) | Compile-time safety and typed numbers. But the toolkit becomes specific to the harness, and every new system or resource changes `model` and every view. | Rejected: the toolkit must work across domains. |
| Typed structs in the workbench, converted to a generic model at the edge | Two representations of the same thing and a mapping layer that adds nothing today. | Rejected (YAGNI). The workbench keeps private decode structs that mirror the JSON contract and maps them straight to entities. |
| Relations as a separate list of `{From, Type, To}` in the model | Symmetric. But the interpreter has to produce two outputs, and a relation could come from an entity that does not exist. | Rejected. Links live on the entity that states them, like the links in the inspected JSON; `Backlinks` gives the other direction. |
| `Health` as an open string | Any vocabulary works. But views cannot color or order unknown values. | Rejected. `Health` is the one closed set; the words of the system stay in `State.Value`. |

## Building the model

| Option | Trade-offs | Decision |
| --- | --- | --- |
| `model.Build(summaries, interpreterOf)` with interpreters selected by the caller | The model knows `source`, so it can record `Evidence` and report failed reads the same way for every caller. The caller decides how targets map to interpreters. | **Chosen.** |
| `model` independent of `source`; callers loop over signals themselves | `model` has no dependencies. But every caller repeats the loop, the evidence and the issue handling. | Rejected. `New` still allows building a model without signals. |
| Select interpreters by target name | Simple. But target names are operator flags (`-source-target name=path`), so renaming a flag would break the model. | Rejected. |
| Select interpreters by the URL path of the target | The paths are the documented HTTP contract of inspected. | **Chosen** for the workbench (`inspectedInterpreter`). |
| Discover interpreters through the links of the index (`/inspected/`) | Closest to "the system tells us about itself". But nothing can be modeled unless the index was read, and a link name has to be mapped to an interpreter anyway. | Rejected for now: the mapping would still be by name, with one more failure mode. |
| Build per page render | No shared state and no invalidation. It costs one decode of each interpreted body per render (at most about 1000 orders). | **Chosen.** |
| Build in a background goroutine after each collection round and cache the result | Cheaper renders. But it adds shared state, locking and a second supervision concern to `Run`. | Rejected (YAGNI, premature optimization). |
| Interpret `/inspected/metrics` with `prometheus/common/expfmt` | Adds counts such as dependency calls. But it needs a new vendored package and numeric properties. | Rejected for this plan: out of scope. |

## Views

| Option | Trade-offs | Decision |
| --- | --- | --- |
| Keep one `view` package | No moves. But raw views depend on `source` and model views on `model`, so one package would depend on both and mix two inputs. | Rejected. |
| Sub-packages by input and visualization: `view/raw`, `view/explorer`, `view/chart`, `view/dashboard`, with shared CSS in `view` | Each package has one input and one purpose, and `go-arch-lint` can enforce the direction. The price is five `Styles` values for a page to include. | **Chosen.** |
| One aggregated `view.Styles` that embeds the CSS of every sub-package | A page includes one stylesheet. But the root package would depend on every view, or embed files owned by other packages. | Rejected. |
| Explorer as one page with every entity as a card and in-page anchors | No routing. But about 1000 order cards are rendered every 2 seconds, and there is no URL per entity. | Rejected. |
| Entity pages at `/model?entity=<id>` | Every entity has a URL, Back works, and a page renders one entity and 100 backlinks at most. | **Chosen.** |
| Charts with a JavaScript chart library | Richer charts. But it is an external dependency or CDN, needs a script that must survive the in-place refresh, and escaping is manual. | Rejected. Charts are server-rendered SVG through html/template. |
| Dashboard, model and raw stacked on one page | No tabs. But the page is very long and every refresh renders everything. | Rejected. The pages are tabs. |

## Returning after a control

| Option | Trade-offs | Decision |
| --- | --- | --- |
| Redirect to the `Referer` header | No form change. But the header can be missing, and a browser may strip the query. | Rejected. |
| Hidden form value `return`, restricted to workbench page paths | Explicit and testable, with no open redirect. | **Chosen.** |
