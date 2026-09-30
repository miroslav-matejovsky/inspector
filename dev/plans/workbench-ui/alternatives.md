# Alternatives

Decisions taken for this plan, with the rejected options and why. Decisions 1 and 2 were made by the project owner.

## 1. What a dashboard shows

| Option | Trade-off | Decision |
| --- | --- | --- |
| Model summary: panels over the observed entity model (state counts, entity lists, pinned entities with root causes) | Uses only what the library already knows. Domain-neutral. Every item links into navigation and explanation. | Chosen |
| Metrics charts (time series of the Prometheus metrics of the inspected service) | Needs metrics ingestion, sample storage and charting. Closer to monitoring than to inspection. | Rejected |

## 2. What is persisted

| Option | Trade-off | Decision |
| --- | --- | --- |
| Dashboard definitions, edited in the UI | Smallest persistence that gives user value. Views stay live. | Chosen |
| Dashboard definitions plus snapshot history | Enables "what changed", but needs a supervised recorder, retention settings and a time dimension in every view. | Rejected |
| Snapshot history only, dashboards in a config file | No UI editing; the history cost remains. | Rejected |

## 3. UI technology

| Option | Trade-off | Decision |
| --- | --- | --- |
| Server-rendered `html/template` pages with links and forms | No build step, no JavaScript, no new tooling. Tested with `httptest` like the JSON views. Contextual escaping protects against source text such as `<script>` in an ID. | Chosen |
| Vanilla JavaScript client of the JSON views | Richer interaction, but the client logic is untested by `task all` unless a JavaScript test runner is added. | Rejected |
| JavaScript framework with a build | Build tooling, dependencies and a second language stack for a development tool. | Rejected |

## 4. Where the Inspector UI lives

| Option | Trade-off | Decision |
| --- | --- | --- |
| Library package `representation` | Every consumer of the library gets the UI. The workbench only embeds it. The boundary check covers it. | Chosen |
| `harness/workbench` | Only the workbench gets a UI; a real consumer would rebuild it. | Rejected |

## 5. URL scheme of the HTML pages

| Option | Trade-off | Decision |
| --- | --- | --- |
| Separate subtree `{prefix}/ui/...` mirroring the JSON paths | Explicit. `curl` keeps getting JSON, browsers get pages. One builder, two link bases. | Chosen |
| Content negotiation on the same URLs (`Accept: text/html`) | One URL per concept, but a browser can no longer show the JSON, responses need `Vary: Accept`, and behavior depends on a header. | Rejected |

## 6. Storage of dashboards

| Option | Trade-off | Decision |
| --- | --- | --- |
| One JSON file, indented, replaced atomically (temporary file and rename) on every write | Standard library only. Human-readable, so the file tells what the dashboards are. Few dashboards, few writes. | Chosen |
| SQLite (`modernc.org/sqlite`) | Transactions and queries nobody needs; a large vendored dependency. | Rejected |
| bbolt | Binary file, not readable; a new dependency. | Rejected |
| One file per dashboard | More files to keep consistent, directory listing as an index. No gain for a handful of dashboards. | Rejected |

## 7. Where the store lives

| Option | Trade-off | Decision |
| --- | --- | --- |
| `representation/dashboard`, stdlib only | Dashboards are a Representation concern. A small package with a pure model and its store. | Chosen |
| New top-level `persistence` package | Breaks "one top-level folder per supporting domain" and invites a generic storage layer nobody needs yet. | Rejected |
| Inside `representation` | Mixes file I/O with HTTP handlers in one package. | Rejected |

## 8. Where panels are evaluated

| Option | Trade-off | Decision |
| --- | --- | --- |
| In `representation`, as view builders | Next to the entity list, overview and explanation builders it reuses. `dashboard` stays pure and stdlib-only. | Chosen |
| In `representation/dashboard` | The package would depend on `observation` and `explanation` and duplicate the builders. | Rejected |

## 9. Live updates

| Option | Trade-off | Decision |
| --- | --- | --- |
| Opt-in `?refresh=<seconds>` rendering `<meta http-equiv="refresh">` | A few lines, testable, no JavaScript. Off by default, so forms are not reloaded while typing. | Chosen |
| JavaScript polling or server-sent events | Smoother, but JavaScript and, for events, a long-lived connection to supervise. | Rejected |
| No live updates | The user reloads by hand; dashboards feel static. | Rejected |

## 10. Inspected panel

| Option | Trade-off | Decision |
| --- | --- | --- |
| Server-rendered operator page with forms; the workbench calls the inspected API over HTTP | Tested in Go like everything else. Makes scenarios reachable without `curl`. Clearly harness, not Inspector. | Chosen |
| JavaScript `fetch` calls to the inspected API | Less Go code, but untested client logic. | Rejected |
| Leave the panel empty | Scenarios stay `curl`-only; the workbench stays uninteresting. | Rejected |

## 11. Protection of form posts

| Option | Trade-off | Decision |
| --- | --- | --- |
| `http.CrossOriginProtection` from the standard library | One wrapper. Rejects cross-site browser posts; `curl` and tests without browser headers pass. | Chosen |
| CSRF tokens in every form | Token generation, storage and checking for a local development tool. | Rejected |
| No protection | Any web page could post to `localhost:8080` and change dashboards or the simulation. | Rejected |

## 12. Dashboard identity

| Option | Trade-off | Decision |
| --- | --- | --- |
| User-chosen slug, `^[a-z0-9][a-z0-9-]{0,62}$` | Readable URLs and file content. Safe in paths without escaping. | Chosen |
| Generated IDs | No naming effort, but opaque URLs and a generator to test. | Rejected |

## 13. Size of an entities panel

| Option | Trade-off | Decision |
| --- | --- | --- |
| Required `limit`, 1 to 100; the panel shows the first `limit` matches in snapshot order, the total and a link to the full list | Bounded page size. Snapshot order is the source's order; the library does not reorder. | Chosen |
| No limit | A kind with a thousand entities fills the dashboard. | Rejected |

## 14. How form fields become a panel

| Option | Trade-off | Decision |
| --- | --- | --- |
| Every form field is copied into the panel; `Validate` rejects fields that do not belong to the type | One code path, strict. The forms send only the fields of their type. | Chosen |
| Read only the fields of the chosen type | Silently ignores input. | Rejected |
