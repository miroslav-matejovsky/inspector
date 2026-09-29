# Alternatives

Decisions taken for this plan, with the rejected options and why.

## 1. Library model

| Option | Trade-off | Decision |
| --- | --- | --- |
| Generic graph: entities (`Ref`, `State`, `Attributes`) and directed named relations in an immutable `Snapshot` | Fits every inspected system. Serves Observe (facts) and Navigate (relations) with one small model. No domain type can reach the library. | Chosen |
| Typed domain plugins (Go generics or interfaces per entity type) | Type safety for one domain, but the library API would be shaped by that domain; every new system changes library types. | Rejected |
| Raw JSON passthrough of source documents | No model to maintain, but navigation and later explanation would have to parse source formats; the library would learn the domain vocabulary. | Rejected |

## 2. Where the domain mapping lives

| Option | Trade-off | Decision |
| --- | --- | --- |
| Consumer-side adapter `harness/adapter` | The only place that knows both the inspected JSON contract and `observation`. The library stays domain-free. Same shape a real consumer of the library will have. | Chosen |
| Declarative mapping configuration inside the library (JSON path rules per kind) | Reusable for JSON APIs, but a mini-language to design, validate and document before the first slice works. | Rejected |
| `harness/inspected` exports `observation` types itself | The inspected system would depend on Inspector; real systems will not do that, so the harness would stop being realistic. | Rejected |

## 3. How the adapter reaches the inspected service

| Option | Trade-off | Decision |
| --- | --- | --- |
| HTTP GET through `connectivity.HTTPReader`, loopback to the workbench listener | Realistic: Inspector is an external, read-only observer. Tests use `httptest.NewServer`. The read-only rule is enforced by the reader API. | Chosen |
| Direct Go calls into `harness/inspected` (`App`, `Simulation.Snapshot`) | No HTTP, but bypasses the public contract and couples the adapter to internals of the simulation. | Rejected |
| In-process `http.RoundTripper` that calls the inspected handler | No socket, but custom transport code in the harness for no behavior gain; the loopback listener already exists. | Rejected |

## 4. Enforcing that the domain does not leak

| Option | Trade-off | Decision |
| --- | --- | --- |
| go-arch-lint for imports plus a vocabulary scan (`task boundary`) | Imports are checked by the existing tool. The scan also catches hardcoded kinds, test fixtures and docs that copy the simulated domain. Both run in `task all`. | Chosen |
| go-arch-lint only | Misses `if kind == "order"` and fixtures that reuse the simulated domain. | Rejected |
| Code review only | Not verifiable, fails silently. | Rejected |
| Go test with `go/parser` checking identifiers and literals | More precise than a text scan, but more code, and it misses docs and comments. | Rejected |

## 5. Attribute values

| Option | Trade-off | Decision |
| --- | --- | --- |
| Strings, formatted by the source adapter | Simplest to validate, encode and display. Enough to observe and navigate. | Chosen |
| Typed values (`any`, or a sum type of string, int, bool, time) | Enables comparisons and charts, which nothing in this plan needs. | Rejected |

## 6. Freshness of observations

| Option | Trade-off | Decision |
| --- | --- | --- |
| Observe on every view request | Always the current state, no goroutine, no lifecycle, no stale data. At most 3 GET requests per view. | Chosen |
| Background polling with a cached snapshot | Faster views, but a goroutine to supervise, a poll interval to configure and stale data between polls. | Rejected |

## 7. First representation format

| Option | Trade-off | Decision |
| --- | --- | --- |
| JSON over HTTP with `href` links | Fully testable with `httptest`, usable by tools and by any later UI. | Chosen |
| HTML views | Visible in the workbench panel, but UI decisions before the model is proven; harder to assert in tests. | Rejected |

## 8. Relations to unknown entities

| Option | Trade-off | Decision |
| --- | --- | --- |
| `NewSnapshot` rejects a relation whose `From` or `To` is not an entity of the snapshot | Fail fast; every navigation link resolves to an entity. | Chosen |
| Keep dangling relations | Tolerates partial sources, but navigation would produce links that lead nowhere. | Rejected |

## 9. HTTP status handling in connectivity

| Option | Trade-off | Decision |
| --- | --- | --- |
| Return every response as a `Document` with its status; the caller decides | Readiness answers 503 with a meaningful JSON body; that is an observation, not a failure. | Chosen |
| Treat every non-2xx status as an error | Inspector would go blind exactly when the inspected system is unhealthy. | Rejected |

## 10. Observation time

| Option | Trade-off | Decision |
| --- | --- | --- |
| Wall-clock `time.Time`, taken by the adapter from an injected `now func() time.Time` | Works for any source; tests pass a fixed clock. | Chosen |
| The source's logical time (ticks) | Simulation vocabulary in the library model. | Rejected |
