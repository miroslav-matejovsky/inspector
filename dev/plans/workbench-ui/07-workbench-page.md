---
title: "07 - Workbench page and documentation"
dependencies: ["05-dashboard-editing", "06-operator-panel"]
effort: "S"
complexity: "low"
---

# 07 - Workbench page and documentation

## Objective

The workbench page at `/` shows the operator page in the inspected panel and the Inspector UI in the inspector panel, each in an iframe with an "open" link for a full tab. An end-to-end test proves the loop the workbench exists for: an outage set through the operator panel is observed and explained on a dashboard created through the Inspector UI. Documentation and the live verification checklist describe the finished state.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/index.html` | Each panel: header with an `open` link and an iframe filling the panel. |
| `harness/workbench/workbench_test.go` | `TestHandlerServesTwoPanels` asserts the iframe sources. |
| `harness/workbench/scenario_test.go` | New `TestOperatorDrivesInspectorDashboard`. |
| `harness/workbench/doc.go` | Panels are no longer placeholders; describe both. |
| `cmd/workbench/main.go` | Start message names the operator page and the Inspector UI. |
| `harness/README.md` | Workbench section: panels, complete flag table, inspector endpoint table (JSON, UI, dashboards), operator endpoint table; manual scenarios mention the operator page. |
| `README.md` (root) | `## Harness` paragraph: what each panel shows. |
| `.todo` | New section `## Workbench UI`. |

## Implementation Tasks

1. Extend `TestHandlerServesTwoPanels`; write `TestOperatorDrivesInspectorDashboard`. Confirm the first fails.
2. Change `index.html`.
3. Update `cmd/workbench/main.go`.
4. Update `doc.go`, `harness/README.md`, root `README.md`, `.todo`.
5. `go test ./harness/workbench/...`, then `task all`.

## Technical Details

### `index.html`

Keep the two-column grid, color tokens and dark mode. Each panel:

```html
<section class="panel" id="inspected">
  <h2>Inspected <a href="/operator/" target="_blank">open</a></h2>
  <iframe class="content" src="/operator/" title="Inspected: operator panel"></iframe>
</section>
<section class="panel" id="inspector">
  <h2>Inspector <a href="/inspector/ui/" target="_blank">open</a></h2>
  <iframe class="content" src="/inspector/ui/" title="Inspector"></iframe>
</section>
```

CSS: `.panel iframe { flex: 1; width: 100%; border: 0; background: var(--panel); }`; `.panel h2 a { float: right; font-weight: 400; text-transform: none; }`. The existing `.panel .content` padding is removed for the iframe. The page stays static; links inside each iframe navigate that iframe only.

### `cmd/workbench/main.go`

```go
fmt.Printf("workbench listening on http://%s (operator at http://%s%s/, inspector UI at http://%s%s/ui/, inspector API at http://%s%s/, inspected API at http://%s%s/)\n",
    cfg.Addr, cfg.Addr, workbench.OperatorPathPrefix, cfg.Addr, workbench.InspectorPathPrefix,
    cfg.Addr, workbench.InspectorPathPrefix, cfg.Addr, inspected.PathPrefix)
```

### `.todo` section

```markdown
## Workbench UI

- Delete `.workbench-dashboards.json` if it exists. Run `task workbench`: the file is created and contains `{"dashboards": []}`.
- Open http://localhost:8080: the left panel shows the operator page; `reload` shows a growing tick. The right panel shows the Inspector overview with kinds `service`, `health_check`, `dependency`, `product` and `order`.
- Right panel, Dashboards: create `ops` with title `Ops`. Add a states panel for `dependency`, a states panel for `order`, an entities panel for `order` in state `failed` with limit 10, and an entity panel for `service` / `inspected`.
- Open the `ops` dashboard and click `auto-refresh every 5 s`.
- Left panel: set `payment-gateway` to `outage`. Within 10 seconds the dashboard shows `dependency` state `outage` with count 1, `service/inspected` in state `down` with root cause `dependency/payment-gateway`, and a growing count of `failed` orders.
- Click the root cause: the dependency page lists incoming `caused_by` and `waits_on` relations from orders.
- Left panel: set `payment-gateway` to `healthy`; pause the clock (the tick stops after `reload`); resume it.
- From an entity page, add the entity to `ops` with `Add to dashboard`: the dashboard shows a new entity panel.
- Press Ctrl+C and run `task workbench` again: `ops` and its panels are still there, and `.workbench-dashboards.json` lists them.
```

### Test (`scenario_test.go`)

`TestOperatorDrivesInspectorDashboard`:

1. `app := startInspected(t)`; `srv := httptest.NewServer(app.Handler())`; `t.Cleanup(srv.Close)`.
2. `inspector, err := workbench.InspectorHandler(srv.URL+inspected.PathPrefix, time.Second, newStore(t))`; `operator, err := workbench.OperatorHandler(srv.URL+inspected.PathPrefix, time.Second)`; `h := workbench.Handler(app.Handler(), inspector, operator)`.
3. Form posts through `h`, each answered 303:
   - `/operator/dependencies/payment-gateway` with `mode=outage`
   - `/inspector/ui/dashboards` with `id=ops&title=Ops`
   - `/inspector/ui/panels` with `dashboard=ops&type=states&kind=dependency`
   - `/inspector/ui/panels` with `dashboard=ops&type=entity&kind=service&entity_id=inspected`
4. `GET /inspector/dashboards/ops` through `h` is 200. Decoded:
   - panel 0 `states.counts` has the elements `{state: outage, count: 1}` and `{state: healthy, count: 1}` (`require.ElementsMatch`);
   - panel 1 `entity`: `observed` true, `state` `down`, `root_causes` equals `[{kind: dependency, id: payment-gateway, state: outage}]` (hrefs ignored in the comparison struct).
5. `GET /inspector/ui/dashboards/ops` through `h` is 200 and `html.UnescapeString` of the body contains `href="/inspector/ui/entities/dependency/payment-gateway"`.

The clock of `startInspected` never fires, so the state changes only through the posts; readiness is derived from the dependency modes at read time.

`workbench_test.go`, `TestHandlerServesTwoPanels`: the body also contains `src="` + `workbench.OperatorPathPrefix` + `/"` and `src="` + `workbench.InspectorPathPrefix` + `/ui/"`.

### Documentation

- `harness/README.md`, section `## workbench`: replace "Both panels are empty for now." with what each panel shows; flag table lists all eight flags; tables of the inspector endpoints (existing JSON, `/ui/` pages, dashboards JSON, form posts) and of the operator endpoints; state that the operator is harness tooling and the only component that changes the simulation from the page, while the inspector only reads it.
- `harness/workbench/doc.go`: the two panels, `# Operator` (from step 06) and the dashboards file.
- Root `README.md`, `## Harness`: "a web page with an inspected panel that shows the operator page for driving the simulation and an inspector panel that shows the Inspector UI".

## Verification

```powershell
go test ./harness/workbench/...
task all
```

Live verification is the `.todo` section above; it is not part of `task all`.

## Acceptance Criteria

- `go test ./harness/workbench/...` exits 0, including `TestOperatorDrivesInspectorDashboard` and the extended `TestHandlerServesTwoPanels`.
- `harness/workbench/index.html` contains `src="/operator/"` and `src="/inspector/ui/"`.
- `Select-String -Path harness/README.md,harness/workbench/doc.go -Pattern 'empty (placeholders|for now)'` prints nothing.
- `.todo` contains `## Workbench UI`.
- `task all` exits 0.

## Non-Goals

- Resizable panels, tabs or layout changes beyond the iframes.
- Running the live verification as part of `task all`.
