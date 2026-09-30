---
title: "03 - Inspected model in the workbench"
dependencies: ["01"]
effort: "M"
complexity: "medium"
---

# 03 - Inspected model in the workbench

## Objective

The workbench defines its model of the inspected system with package `model`. That means:

- the kinds, link types and health mapping of the inspected system;
- one `model.Interpreter` per observation endpoint of inspected;
- the selector `inspectedInterpreter`, which picks the interpreter by the URL path of a target.

The interpreters read only the collected signals, that is the JSON bodies of the documented HTTP contract. They do not import `inspected/fulfillment`, `inspected/health` or `inspected/simulation`.

Contract tests build the model from the responses of a running inspected app. They prove that every link in the model resolves. The pages use the model in step 07; until then the interpreters are staged in the deadcode allowlist.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/inspectedmodel.go` | new: kinds, link types, health maps, interpreters, `inspectedInterpreter`, `decodeBody`, `appendLink` |
| `harness/workbench/export_test.go` | new: `InspectedModel` for the tests of package `workbench_test` |
| `harness/workbench/inspectedmodel_test.go` | new: contract tests (package `workbench_test`) |
| `harness/workbench/doc.go` | new section `# Inspected model` |
| `.go-arch-lint.yml` | `workbench` may depend on `model` |
| `taskfile/deadcode.ps1` | staged names of the new unreachable functions |

## Implementation Tasks

1. Create `harness/workbench/export_test.go`:

   ```go
   package workbench

   import (
   	"github.com/miroslav-matejovsky/inspector/model"
   	"github.com/miroslav-matejovsky/inspector/source"
   )

   // InspectedModel builds the model of the inspected system from summaries,
   // as the pages do. Only tests use it.
   func InspectedModel(summaries []source.TargetSummary) *model.Model {
   	return model.Build(summaries, inspectedInterpreter)
   }
   ```

2. Write `harness/workbench/inspectedmodel_test.go` with the helpers and tests under Verification. At this point they do not compile.
3. Create `harness/workbench/inspectedmodel.go` with the code under Technical Details.
4. Run `go test ./harness/workbench/...` until it passes.
5. Add `model` to `workbench.mayDependOn` in `.go-arch-lint.yml`.
6. Add the section under "Package documentation" to `harness/workbench/doc.go`, after `# Source`.
7. Run `deadcode ./cmd/...` and add every newly printed name for `harness/workbench/inspectedmodel.go` to `$allow` in `taskfile/deadcode.ps1`, under the comment `# staged by dev/plans/model, removed in step 07`.
8. Run `task all`.

## Technical Details

### Vocabulary

```go
// Kinds of the entities of the inspected system.
const (
	kindService    = "service"    // the index: /inspected/
	kindReadiness  = "readiness"  // the readiness report: /inspected/health/ready
	kindCheck      = "check"      // one check of the readiness report
	kindDependency = "dependency" // a downstream system of inspected
	kindProduct    = "product"
	kindOrder      = "order"
)

// Link types, read from the entity that holds the link.
const (
	linkReadiness    = "readiness"     // service -> readiness
	linkCheck        = "check"         // readiness -> check
	linkDeterminedBy = "determined by" // check -> dependency or product ("causes")
	linkProduct      = "product"       // order -> product ("links.product")
	linkWaitsOn      = "waits on"      // order -> dependency ("links.waiting_on")
	linkDecidedBy    = "decided by"    // order -> dependency or product (history "cause")
)

// Health of the status words of inspected. A word that is missing maps to
// the zero Health, which model.New reads as unknown.
var (
	statusHealth = map[string]model.Health{"up": model.HealthOK, "degraded": model.HealthDegraded, "down": model.HealthDown}
	modeHealth   = map[string]model.Health{"healthy": model.HealthOK, "slow": model.HealthDegraded, "outage": model.HealthDown}
	orderHealth  = map[string]model.Health{
		"pending": model.HealthOK, "paid": model.HealthOK, "shipped": model.HealthOK, "failed": model.HealthDown,
	}
)
```

### Selection

```go
// inspectedInterpreters maps the URL path of a target to the interpreter of
// its body. The paths are observation endpoints of package inspected.
var inspectedInterpreters = map[string]model.Interpreter{
	inspected.PathPrefix + "/":                 interpretIndex,
	inspected.PathPrefix + "/health/ready":     interpretReadiness,
	inspected.PathPrefix + "/api/dependencies": interpretDependencies,
	inspected.PathPrefix + "/api/products":     interpretProducts,
	inspected.PathPrefix + "/api/orders":       interpretOrders,
}

// inspectedInterpreter returns the interpreter of t by the path of its URL,
// or nil when the path is not in inspectedInterpreters. The query is
// ignored.
func inspectedInterpreter(t source.Target) model.Interpreter {
	u, err := url.Parse(t.URL)
	if err != nil {
		// source.Config.Validate accepts only URLs that parse.
		return nil
	}
	return inspectedInterpreters[u.Path]
}
```

### Helpers

```go
// decodeBody decodes the JSON body of sig into v when the status of sig is
// one of ok. Unknown fields are ignored: inspected may state more than the
// model reads.
func decodeBody(sig source.Signal, v any, ok ...int) error {
	if !slices.Contains(ok, sig.StatusCode) {
		return fmt.Errorf("status %d", sig.StatusCode)
	}
	if err := json.Unmarshal(sig.Body, v); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	return nil
}

// appendLink appends the link {typ, to} to links, unless to is empty or
// links already holds it.
func appendLink(links []model.Link, typ, to string) []model.Link {
	l := model.Link{Type: typ, To: to}
	if to == "" || slices.Contains(links, l) {
		return links
	}
	return append(links, l)
}
```

### Interpreters

Every interpreter decodes into an anonymous struct that mirrors the part of the JSON contract it reads (see `harness/inspected/resources.go` for the contract; do not import it).

| Interpreter | Accepted status | Entities |
| --- | --- | --- |
| `interpretIndex` | 200 | one `service`: ID `links.self`, Name `service`, property `description`, link `readiness` -> `links.health_ready` |
| `interpretReadiness` | 200, 503 | one `readiness`: ID = path of `sig.URL`, Name `readiness`, State `{Value: status, Health: statusHealth[status]}`, one link `check` per check; then one `check` per check, in body order: ID `<readiness ID>#<name>`, Name `name`, State `{Value: status, Health: statusHealth[status], Reason: reason}`, one link `determined by` per entry of `causes` |
| `interpretDependencies` | 200 | one `dependency` per entry: ID `links.self`, Name `name`, State `{Value: mode, Health: modeHealth[mode]}` |
| `interpretProducts` | 200 | one `product` per entry: ID `links.self`, Name `sku`, properties `name`, `price_cents`, `stock`, `capacity`, `reorder_point` in this order; no State; no links |
| `interpretOrders` | 200 | one `order` per entry, newest first: ID `links.self`, Name `id`, State `{Value: status, Health: orderHealth[status], Reason: failure_reason}`, properties (below), links (below) |

Property values are the JSON values as text: `strconv.Itoa`, `strconv.FormatInt` and `strconv.FormatUint` for numbers, strings as they are.

The order interpreter, in full:

```go
func interpretOrders(sig source.Signal) ([]model.Entity, error) {
	var body struct {
		Orders []struct {
			ID            string `json:"id"`
			SKU           string `json:"sku"`
			Quantity      int    `json:"quantity"`
			TotalCents    int64  `json:"total_cents"`
			Channel       string `json:"channel"`
			Status        string `json:"status"`
			PlacedAtTick  uint64 `json:"placed_at_tick"`
			UpdatedAtTick uint64 `json:"updated_at_tick"`
			FailureReason string `json:"failure_reason"`
			History       []struct {
				From   string `json:"from"`
				To     string `json:"to"`
				AtTick uint64 `json:"at_tick"`
				Reason string `json:"reason"`
				Cause  string `json:"cause"`
			} `json:"history"`
			Links struct {
				Self      string `json:"self"`
				Product   string `json:"product"`
				WaitingOn string `json:"waiting_on"`
			} `json:"links"`
		} `json:"orders"`
	}
	if err := decodeBody(sig, &body, http.StatusOK); err != nil {
		return nil, err
	}
	entities := make([]model.Entity, 0, len(body.Orders))
	// The list is in placement order. Newest first, so that a view that shows
	// the first entities of a kind shows the recent orders.
	for _, o := range slices.Backward(body.Orders) {
		e := model.Entity{
			ID:   o.Links.Self,
			Kind: kindOrder,
			Name: o.ID,
			State: model.State{Value: o.Status, Health: orderHealth[o.Status], Reason: o.FailureReason},
			Properties: []model.Property{
				{Name: "sku", Value: o.SKU},
				{Name: "quantity", Value: strconv.Itoa(o.Quantity)},
				{Name: "total_cents", Value: strconv.FormatInt(o.TotalCents, 10)},
				{Name: "channel", Value: o.Channel},
				{Name: "placed_at_tick", Value: strconv.FormatUint(o.PlacedAtTick, 10)},
				{Name: "updated_at_tick", Value: strconv.FormatUint(o.UpdatedAtTick, 10)},
			},
		}
		for _, h := range o.History {
			step := h.To + " (" + h.Reason + ")"
			if h.From != "" {
				step = h.From + " -> " + step
			}
			e.Properties = append(e.Properties, model.Property{Name: "tick " + strconv.FormatUint(h.AtTick, 10), Value: step})
		}
		e.Links = appendLink(e.Links, linkProduct, o.Links.Product)
		e.Links = appendLink(e.Links, linkWaitsOn, o.Links.WaitingOn)
		for _, h := range o.History {
			e.Links = appendLink(e.Links, linkDecidedBy, h.Cause)
		}
		entities = append(entities, e)
	}
	return entities, nil
}
```

The readiness interpreter takes its ID from the signal URL, so that the `readiness` link of the service (`links.health_ready`, a path) resolves:

```go
u, err := url.Parse(sig.URL)
if err != nil {
	return nil, fmt.Errorf("parse url: %w", err)
}
id := u.Path // "/inspected/health/ready"
```

Entity IDs are the paths that inspected states as self links, for example `/inspected/api/dependencies/payment-gateway`. The `causes`, `waiting_on`, `product` and `cause` links use the same paths, so they resolve without joining URLs. An entry without a self link produces an entity with an empty ID, which `model.New` reports as an issue.

### Package documentation

Add to `harness/workbench/doc.go`, after `# Source`:

```go
// # Inspected model
//
// The workbench models the inspected system with package model, from the
// signals of its Source only. A target is interpreted when the path of its
// URL is one of these observation endpoints of inspected; other targets,
// such as the metrics, are not modeled:
//
//	/inspected/                  service: name, description; link readiness
//	/inspected/health/ready      readiness and one check per check; links check, determined by
//	/inspected/api/dependencies  dependency per dependency, state = mode
//	/inspected/api/products      product per product with its stock numbers
//	/inspected/api/orders        order per order, newest first; links product, waits on, decided by
//
// Entity IDs are the paths that inspected states as self links, a check is
// "<readiness path>#<check name>". Health follows the words of inspected:
// up/healthy is ok, degraded/slow is degraded, down/outage is down; an order
// is down when it failed and ok otherwise; products state no health.
// Status 503 is accepted for readiness only; any other status, or a body
// that is not the expected JSON, is a model issue of the target.
```

## Verification

Commands:

```powershell
go test ./harness/workbench/... -run InspectedModel -v
go-arch-lint check
deadcode ./cmd/...
task all
```

Helpers in `inspectedmodel_test.go`:

```go
// read returns the summary that the Source would store for target name
// after reading path from h at t0.
func read(t *testing.T, h http.Handler, name, path string) source.TargetSummary

// send serves method path with the JSON body by h and requires status want.
func send(t *testing.T, h http.Handler, method, path, body string, want int) *httptest.ResponseRecorder

// interpretedTargets are the five modeled targets, name -> path, as in Taskfile.yml.
var interpretedTargets = [][2]string{
	{"index", "/inspected/"}, {"health_ready", "/inspected/health/ready"},
	{"dependencies", "/inspected/api/dependencies"}, {"products", "/inspected/api/products"},
	{"orders", "/inspected/api/orders"},
}

// modelOf reads every interpreted target of h and builds the model.
func modelOf(t *testing.T, h http.Handler) *model.Model
```

`read` builds `source.Signal{Target: name, URL: "http://workbench.test" + path, ObservedAt: t0, StatusCode: rec.Code, ContentType: rec.Header().Get("Content-Type"), Body: rec.Body.Bytes()}`. The app comes from `startInspected(t)`, whose clock never ticks during a test; `POST /inspected/sim/advance` moves time.

| Test | Setup | Asserts |
| --- | --- | --- |
| `TestInspectedModelReadsService` | none | entity `/inspected/` has kind `service`, name `inspected`, property `description` = `Simulated order fulfillment service`, links `[{readiness /inspected/health/ready}]` |
| `TestInspectedModelReadsDependencies` | `PUT /inspected/sim/dependencies/warehouse {"mode":"slow"}` -> 200 | `/inspected/api/dependencies/payment-gateway`: value `healthy`, health ok; `/inspected/api/dependencies/warehouse`: value `slow`, health degraded |
| `TestInspectedModelReadsReadiness` | outage of payment-gateway | the readiness signal has status 503; `/inspected/health/ready` has value `down`, health down, and a `check` link to `/inspected/health/ready#payment-gateway`, `#warehouse` and `#inventory`; check `#payment-gateway` has health down, reason `payment-gateway is in outage: calls fail` and link `{determined by /inspected/api/dependencies/payment-gateway}` |
| `TestInspectedModelReadsProducts` | none | `/inspected/api/products/sku-001` has kind `product`, name `sku-001`, property names `[name price_cents stock capacity reorder_point]`, health unknown, no links |
| `TestInspectedModelReadsOrders` | `POST /inspected/api/orders {"sku":"sku-001","quantity":1}` -> 201, ID from the `Location` header | the order has value `pending`, health ok, links `[{product /inspected/api/products/sku-001} {waits on /inspected/api/dependencies/payment-gateway}]` and property `tick 0` = `pending (order_placed)` |
| `TestInspectedModelReadsFailedOrder` | outage of payment-gateway, place an order, `POST /inspected/sim/advance {"ticks":3}` -> 200 | the order has value `failed`, health down, reason `payment_gateway_unavailable`, a link `{decided by /inspected/api/dependencies/payment-gateway}` and no `waits on` link |
| `TestInspectedModelListsOrdersNewestFirst` | place two orders | the first `order` entity in `Entities()` is the second order |
| `TestInspectedModelLinksResolve` | outage of payment-gateway, place an order, advance 3 ticks | for every entity and every link, `m.Entity(link.To)` is found; `m.Issues()` is empty |
| `TestInspectedModelIgnoresOtherTargets` | read `health_live` and `metrics` only | `Entities()` and `Issues()` are empty |
| `TestInspectedModelReportsUnexpectedStatus` | a products summary with status 503 and body `{}` | `Issues()` is `[{products "interpret: status 503"}]` |
| `TestInspectedModelReportsInvalidBody` | an orders summary with status 200 and body `not json` | one issue of target `orders` that starts with `interpret: decode body:` |
| `TestInspectedModelReadsUnknownWordsAsUnknownHealth` | a dependencies summary with body `{"dependencies":[{"name":"x","mode":"flaky","links":{"self":"/x"}}]}` | entity `/x` has value `flaky` and health unknown |

## Acceptance Criteria

- Every test in the table passes.
- `git grep -n "inspected/fulfillment\|inspected/health\|inspected/simulation" -- harness/workbench/inspectedmodel.go` finds nothing.
- `.go-arch-lint.yml` lists `model` in `workbench.mayDependOn`, and `go-arch-lint check` passes.
- `harness/workbench/doc.go` has the section `# Inspected model`.
- `task all` passes.

## Non-Goals

- No page shows the model yet (step 07).
- No interpretation of `/inspected/metrics`, `/inspected/health/live` or `/inspected/sim`.
- No merging of one entity from several signals, for example a dependency from both `/api/dependencies` and `/sim`.
- No product health derived from stock numbers: inspected states stock problems through the inventory check and its `causes`.
