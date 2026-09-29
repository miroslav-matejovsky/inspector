# Assessment

## Feasibility

High. Every new package uses only the standard library and `testify` in tests. No new module. `observation` and `navigation` are pure functions over small data. `connectivity`, `representation` and `harness/adapter` are tested with `httptest`; the adapter contract test runs the real inspected app in-process with a clock that never fires during the test, as the existing `harness/inspected` tests do.

Checked while writing the plan (probes in a scratch module, not in the repository):

- Toolchain is `go1.27.1`. `golangci-lint` is `2.14.0`.
- go-arch-lint with the repository settings (`deepScan: true`):
  - A component without a `deps` entry that imports a harness package fails with `Component lib shouldn't depend on .../harness/sim`.
  - A Go file in a folder that is not mapped to any component fails with `File ... not attached to any component in archfile`. A new library package cannot bypass the rules by being left out of `.go-arch-lint.yml`.
  - `workbench` injecting an adapter value into a representation function that takes a consumer-defined interface passes with `workbench: { mayDependOn: [adapter, representation] }`; no extra rule for `observation` or between `representation` and `adapter` is needed.
- `http.ServeMux` pattern `GET /inspector/entities/{kind}/{id}` matches `/inspector/entities/k%20ind/a%2Fb%20c` and `PathValue` returns `k ind` and `a/b c`. IDs with `/` work when escaped with `url.PathEscape`.
- `golangci-lint` reports `defer resp.Body.Close()` as `errcheck`. Step 04 closes the body explicitly and joins the error.
- The script of step 01 was run on a clean folder (exit 0), on a folder with `const k = "order"` (exit 1, line reported), on a missing folder (exit 1) and without arguments (exit 1). On the current library folders it exits 0.
- The inspected catalog has `sku-001` with stock 40. Stock changes only on ticks, so a placed order stays `pending` and stock stays 40 while the clock does not fire.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| `task deadcode` only loads packages reachable from `cmd/...`. Library functions become visible to it in step 07 when the workbench imports them. | `task all` fails late in step 07. | Every exported function in the plan has a named production caller (listed in step 07). Step 07 removes or wires anything reported; no allowlist entries. |
| The vocabulary scan reports a false positive, for example "product" in an unrelated English sentence. | `task boundary` fails. | Rephrase the library text. Changing the pattern requires updating the table in the script header and in step 01. |
| The workbench inspector reads the inspected service through its own listener. During shutdown the inner request can fail. | A view request during shutdown answers 503. | Acceptable. Every read is bounded by `-inspector-source-timeout`; `Run` shutdown behavior is unchanged. |
| The adapter builds one snapshot from 3 GET requests that are not atomic. The clock can tick between them. | Readiness and orders can come from different ticks. | Documented in `harness/adapter/doc.go`. Products are a fixed catalog, so `for_product` relations always resolve. |
| Up to 1000 retained orders. `navigation.Links` scans all relations. | Linear work per view request. | About 1010 relations; negligible. No index until a measurement shows a need. |
| The adapter keeps its own copy of the inspected JSON contract. The contract can drift. | Wrong or missing entities. | `TestObserveInspectedService` (step 05) runs the adapter against the real inspected handler. Missing required fields fail with an error. |

## Dependencies

- No external module. Tools already used by `task all`: `gotestsum`, `deadcode`, `go-arch-lint`, `golangci-lint`, `pwsh`.
- Live verification (`.todo`) needs `curl` and a browser. Not needed for `task all`.
- Step order: 01; then 02 and 04 in any order; 03 after 02; 05 after 02 and 04; 06 after 02 and 03; 07 last.

## Validation

- Per step: the listed `go test` command, `go-arch-lint check`, `task boundary` and `task all`.
- Boundary: `task boundary` in `task all`; negative probe in step 01; `go list -deps` check in the plan success criteria.
- Contract with the inspected service: `TestObserveInspectedService` (step 05).
- End to end in process: `TestInspectorHandlerObservesInspected` (step 07).
- Live behavior: `.todo` section `Inspector in the workbench` (step 07), executed manually outside `task all`.

## Rollback

Nothing is committed by the implementer. To roll back the whole plan:

1. Delete `observation/*.go`, `navigation/*.go`, `connectivity/*.go`, `representation/*.go`, `harness/adapter/`, `taskfile/boundary.ps1`.
2. `git restore observation/README.md navigation/README.md connectivity/README.md representation/README.md`.
3. `git restore harness/workbench cmd/workbench Taskfile.yml .go-arch-lint.yml README.md harness/README.md .todo`.
4. `task all` passes with the previous state.

Rolling back a single step means reverting only the files that step lists under Target Artifacts, in reverse step order.
