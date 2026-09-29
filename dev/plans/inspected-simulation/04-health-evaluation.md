---
title: "04 - Health evaluation"
dependencies: ["01-fulfillment-domain"]
effort: "S"
complexity: "low"
---

# 04 - Health evaluation

## Objective

Package `harness/inspected/health` evaluates a `fulfillment.Snapshot` into a readiness `Report`. Evaluation is a pure function. Every check that is not `up` carries a human-readable reason that names its cause, so a future Inspector can explain readiness from the report alone.

## Target Artifacts

| File | Content |
| --- | --- |
| `harness/inspected/health/doc.go` | Package doc: checks, status rules, reason formats. |
| `harness/inspected/health/health.go` | `Status`, `Check`, `Report`, `CheckInventory`, `Evaluate`. |
| `harness/inspected/health/health_test.go` | Tests. |
| `.go-arch-lint.yml` | Component `health`, rule `mayDependOn: [fulfillment]`. |

## Implementation Tasks

1. Write `health_test.go` with every test listed below. Confirm it fails to compile.
2. Create `health.go`.
3. Create `doc.go`.
4. In `.go-arch-lint.yml` add component `health: { in: harness/inspected/health }` and under `deps:` add `health: { mayDependOn: [fulfillment] }`.
5. Run `go test ./harness/inspected/health/...` until it passes, then `task all`.

## Technical Details

### API

```go
type Status string

const (
    StatusUp       Status = "up"
    StatusDegraded Status = "degraded"
    StatusDown     Status = "down"
)

// CheckInventory is the name of the stock check.
const CheckInventory = "inventory"

type Check struct {
    Name   string // dependency name or CheckInventory
    Status Status
    Reason string // empty when Status is up
}

type Report struct {
    Status Status  // worst status of all checks
    Checks []Check // dependencies in snapshot order, then inventory
}

func Evaluate(s fulfillment.Snapshot) Report
```

### Rules

One check per `Snapshot.Dependencies` entry, named by the dependency name:

| Mode | Status | Reason |
| --- | --- | --- |
| `healthy` | `up` | empty |
| `slow` | `degraded` | `fmt.Sprintf("%s is slow: calls take %d ticks", name, fulfillment.SlowLatencyTicks)` |
| `outage` | `down` | `fmt.Sprintf("%s is in outage: calls fail", name)` |

One `inventory` check:

| Condition | Status | Reason |
| --- | --- | --- |
| every product has `Stock > 0` | `up` | empty |
| at least one product has `Stock == 0` | `degraded` | `"out of stock: "` + SKUs with zero stock in SKU order joined by `", "` |

Overall `Report.Status`: `down` if any check is `down`, else `degraded` if any check is `degraded`, else `up`.

Readiness HTTP mapping (implemented in step 05): `down` gives 503, `up` and `degraded` give 200.

### Tests (`health_test.go`, package `health_test`)

Snapshots are built with `fulfillment.NewState` and `SetDependencyMode`, or as `fulfillment.Snapshot` literals for stock cases.

| Test | Input | Expected `Report` |
| --- | --- | --- |
| `TestEvaluateAllHealthy` | Standard catalog, both dependencies `healthy` | `up`; checks `[{payment-gateway, up, ""}, {warehouse, up, ""}, {inventory, up, ""}]` |
| `TestEvaluateSlowDependency` | `payment-gateway` `slow` | `degraded`; first check reason `payment-gateway is slow: calls take 3 ticks` |
| `TestEvaluateOutage` | `warehouse` `outage` | `down`; second check `{warehouse, down, "warehouse is in outage: calls fail"}` |
| `TestEvaluateOutageWinsOverDegraded` | `payment-gateway` `slow`, `warehouse` `outage` | `down` |
| `TestEvaluateOutOfStock` | Snapshot literal with `sku-002` and `sku-005` at stock 0 | `degraded`; inventory reason `out of stock: sku-002, sku-005` |
| `TestEvaluateIsPure` | Evaluate the same snapshot twice | Equal reports; input snapshot unchanged (`require.Equal` against a copy taken before) |

## Verification

```powershell
go test ./harness/inspected/health/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/inspected/health/...` exits 0 and runs every test listed above.
- `harness/inspected/health` imports only stdlib and `harness/inspected/fulfillment`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- HTTP endpoints and JSON encoding (step 05).
- Liveness. Liveness is "the actor answers" and is decided in step 05.
- Health history or flapping detection.
