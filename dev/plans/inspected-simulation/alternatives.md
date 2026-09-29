# Alternatives

Decisions taken for this plan, with the rejected options and why.

## 1. Simulated domain

| Option | Trade-off | Decision |
| --- | --- | --- |
| Order fulfillment | Products, orders and two dependencies give relationships (order to product, stage to dependency), business and system failures, and causes that can be explained. Familiar to any developer. | Chosen |
| Job queue and worker pool | Strong metrics story, but few relationships between concepts and little to explain beyond "queue is long". | Rejected |
| IoT device fleet | Rich telemetry, but state is mostly independent per device; weak relationships and causality. | Rejected |

## 2. State ownership and concurrency

| Option | Trade-off | Decision |
| --- | --- | --- |
| Plain Go owner goroutine with typed messages | One owner per state, message passing, deterministic tests without a framework. Same actor semantics as `AGENTS.md` requires. No new dependency. | Chosen |
| ergo actors | Full supervision tree and ergo test tooling, but needs a node inside the workbench and an HTTP bridge (meta web handler) for every request. Large setup for a harness. | Rejected |
| Mutex-guarded struct | Least code, but shared mutable state accessed from every HTTP goroutine; conflicts with "avoid mutexes when actor ownership can solve the problem". | Rejected |

## 3. Time model

| Option | Trade-off | Decision |
| --- | --- | --- |
| Logical ticks, wall-clock ticker only drives the clock | Every rule is testable and reproducible without sleeps. Scenarios can be stepped exactly. | Chosen |
| Wall-clock durations inside the domain | More realistic latencies, but tests need fake clocks or sleeps, and scenarios are not reproducible. | Rejected |

## 4. Clock integration

| Option | Trade-off | Decision |
| --- | --- | --- |
| Actor `select` reads an injected `<-chan time.Time` | No extra goroutine. Tests pass their own channel or nil. | Chosen |
| Separate clock goroutine sending tick messages | One more goroutine with its own lifecycle and shutdown ordering. | Rejected |

## 5. Randomness

| Option | Trade-off | Decision |
| --- | --- | --- |
| Seeded `math/rand/v2` PCG owned by the actor | Reproducible traffic per seed; domain stays pure. | Chosen |
| Randomness inside the domain | Domain tests would depend on seeds. | Rejected |
| `crypto/rand` or unseeded source | Not reproducible. | Rejected |

## 6. Metrics library

| Option | Trade-off | Decision |
| --- | --- | --- |
| `prometheus/client_golang` with a dedicated registry | Standard, boring, correct exposition format, `testutil` and `promlint` for tests. One dependency tree. | Chosen |
| Hand-written text exposition | No dependency, but re-implements escaping, histograms and content negotiation; easy to get wrong. | Rejected |
| Global default registry (`promauto`) | Global mutable state; breaks multiple apps in one test binary. | Rejected |
| OpenTelemetry SDK with Prometheus exporter | Much larger dependency tree; adds nothing the harness needs. | Rejected |

## 7. Gauge update strategy

| Option | Trade-off | Decision |
| --- | --- | --- |
| Actor sets gauges from a snapshot after every mutation | Scrape never touches the actor; gauges always match the last committed state. | Chosen |
| Custom collector that queries the actor at scrape time | Scrape becomes a synchronous call into the actor; scrape latency depends on mailbox health. | Rejected |

## 8. Health format

| Option | Trade-off | Decision |
| --- | --- | --- |
| Own JSON: `status` `up`, `degraded`, `down`, checks with `name`, `status`, `reason` | Small, documented, reason field serves explanation directly. | Chosen |
| IETF draft `application/health+json` (`pass`, `warn`, `fail`) | Draft standard, nested and verbose structure; the draft never became an RFC. | Rejected |
| Plain status code without body | Nothing to explain. | Rejected |

## 9. Hosting

| Option | Trade-off | Decision |
| --- | --- | --- |
| Mounted in the workbench server under `/inspected` | One process, one port, one entry point; isolation by path prefix. Required by the task. | Chosen |
| Separate entry point and port | Closer to a real deployment, but more wiring and explicitly not wanted. | Rejected |

## 10. Scenario control

| Option | Trade-off | Decision |
| --- | --- | --- |
| HTTP control API under `/inspected/sim` (clock, advance, dependency modes) | Scenarios are reproducible from tests and by hand; control is separated from the business API by path. | Chosen |
| Autonomous simulation only | No way to create a known cause on demand. | Rejected |
| Faults configured by flags at startup | Cannot change a fault while running; every scenario needs a restart. | Rejected |
