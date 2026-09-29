# Assessment

## Feasibility

High. Every part uses the standard library except the Prometheus client. The domain is small (5 products, 2 dependencies, 4 order statuses). All behavior is testable with deterministic unit tests: the domain is pure, the actor takes an injected clock channel, and HTTP handlers run through `httptest.NewRecorder` without opening ports.

Checked while writing the plan:

- `github.com/prometheus/client_golang` latest release is `v1.24.1` (`go list -m -versions`).
- Go toolchain is `go1.27.1`; `math/rand/v2`, `ServeMux` method patterns and `PathValue` are available.
- A probe module with the repository `.go-arch-lint.yml` settings (`deepScan: true`) accepted an actor package that receives a recorder implementation from a sibling package through a consumer-defined interface, with no extra `mayDependOn` entry.
- In the same probe, `deadcode` did not report unexported marker methods (`isEvent`) of types that are converted to their interface. No allowlist entries are expected.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| `deadcode` reports functions only once `cmd/workbench` imports the new packages (step 07). Unused functions from steps 01 to 06 surface late. | `task all` fails in step 07. | Every exported function in the plan has a named production caller. Step 07 task 8 removes or wires anything reported; no allowlist. |
| Memory growth in a long-running workbench. | Unbounded order list. | `MaxRetainedOrders = 1000`, oldest terminal orders pruned each tick (step 01, tested). |
| `Advance` of 1000 ticks blocks the actor mailbox. | Other requests wait. | Bounded by `MaxAdvanceTicks`; every HTTP request has `RequestTimeout` and returns 503 on timeout. |
| Process collector behaves differently on Windows and Linux. | Platform-specific test failures. | Tests assert only `go_goroutines` and `fulfillment_*` and `inspected_*` metrics. |
| Label cardinality growth. | Large metric output. | All label values come from closed enumerations or the fixed 5-product catalog. |
| Transitive dependencies of `client_golang` (`client_model`, `common`, `procfs`, `protobuf`, `x/sys`, and others). | Larger `go.sum`, supply chain surface. | Single pinned version; `task deps` and `govulncheck` (installed by `task go-tools`) cover updates and known vulnerabilities. |
| At-most-once request semantics: a timed-out `POST /inspected/api/orders` may still place the order. | Client sees 503 but the order exists. | Documented in `simulation/doc.go` and `harness/inspected/doc.go`; acceptable for a simulation. |

## Dependencies

- External module: `github.com/prometheus/client_golang v1.24.1` (step 03).
- Tools already used by `task all`: `gotestsum`, `deadcode`, `go-arch-lint`, `golangci-lint`.
- Live verification (`.todo`) needs `curl`, `promtool` and a local Prometheus. Not needed for `task all`.
- Step order: 01, then 02, 03 and 04 in any order, then 05, 06, 07.

## Validation

- Per step: the listed `go test` command and `task all`.
- Architecture: `go-arch-lint check` after each step that adds a component.
- Metrics hygiene: `TestMetricsPassLint` (`promlint`).
- End-to-end behavior: scenario tests in step 06 drive the full HTTP surface in-process.
- Live behavior: the `.todo` section `Inspected simulation` (step 07), executed manually outside `task all`.

## Rollback

Nothing is committed by the implementer. To roll back:

1. Delete `harness/inspected/` except the empty directory, and `harness/workbench/config.go`, `harness/workbench/config_test.go`.
2. `git restore harness/workbench cmd/workbench Taskfile.yml .go-arch-lint.yml go.mod go.sum harness/README.md README.md .todo`.
3. `task all` passes with the previous workbench.

Rolling back a single step means reverting only the files that step lists under Target Artifacts; later steps depend on earlier ones, so roll back in reverse step order.
