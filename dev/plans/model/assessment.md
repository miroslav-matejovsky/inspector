# Assessment

## Feasibility

The plan builds on code that exists and works:

- `source.Source.Summary` already returns the latest signal of every target with its body, which is exactly the input of `model.Build`. `source` needs no change.
- inspected already states identity and relations in its JSON: self links, `causes`, `links.waiting_on`, `links.product` and history `cause`. The interpreters only map them; no relation is inferred.
- The workbench already renders views with html/template, refreshes the panel in place and keeps scroll positions. The new pages reuse that machinery.
- No new third-party dependency is needed. The chart is SVG from html/template.

The largest step is 07 (L). It touches routing, the template, the script and the controls. Every other step is S or M and adds one package or one file set.

## Risks

| Risk | Likelihood | Impact | Mitigation |
| --- | :---: | :---: | --- |
| `task all` fails in steps 01 to 06 because `deadcode` reports functions that no command reaches yet | High | Medium | Each step stages its names in the allowlist of `taskfile/deadcode.ps1`. That script allows "functions staged for future use". Step 07 empties the list, and its acceptance criteria check that. |
| The JSON contract of inspected changes and the interpreters break silently | Medium | Medium | The contract tests of step 03 read real responses of a running inspected app. `TestInspectedModelLinksResolve` fails when any link stops resolving. A broken body becomes a visible model issue on the dashboard, not a crash. |
| Page weight: about 1000 retained orders plus the orders in progress are rendered every 2 seconds | Medium | Medium | The explorer shows 100 rows per kind and 100 backlinks. The dashboard shows 50 entities. The raw view already limits bodies to 500 lines. `.todo` has a live check with more than 1000 orders. |
| Build cost per render (decoding the orders list, about 1 MB, twice a second per open tab) | Low | Low | Both are bounded by the retention of inspected (`MaxRetainedOrders` 1000). If the live check in `.todo` shows lag, the fix is a cache, which is a separate change and a new plan. |
| Open redirect through the `return` form value | Low | High | `returnPath` accepts only paths of workbench pages, without scheme or host. `TestDependencyControlRejectsInvalidReturn` covers another host, a scheme-relative URL, a backslash path, an unknown path and an empty value. |
| html/template handles SVG `<title>` as RCDATA | Low | Low | The title holds only escaped text. `TestHealthByKindEscapesKinds` checks the escaping in labels and titles. |
| The refresh script loses the page query | Low | Medium | The script fetches `location.pathname + location.search`. This is covered only live (`.todo`), because Go tests do not run the script. |
| Entity IDs contain characters that break URLs | Low | Low | IDs go into URLs only through `url.QueryEscape`. Tests cover IDs with `/`. |

## Dependencies

- Internal: `source` (unchanged API), `harness/inspected` (its JSON contract and `PathPrefix`), and the page and script of the workbench.
- Tools: `go-arch-lint`, `deadcode`, `golangci-lint` and `gotestsum`, all through `task all`.
- External: none new.

## Validation

- TDD in every step: the tests of a step are written first and named in the step file.
- `task all` passes after every step: clean, tidy, fmt, vet, deadcode, arch-lint, lint and test.
- Contract tests (step 03) run the real inspected app in process and check the interpreters against its responses.
- Workbench tests (step 07) run the real inspected app and a real Source against it, and check every page, tab, return path and status.
- Live checks that need a browser (script, colors, performance) are listed in `.todo` by step 07, following the rule that tests never start real systems.

## Rollback

Each step is a separate working-tree change that the user commits.

- Steps 01, 03, 04, 05 and 06 only add packages and files. Reverting one removes the package and its allowlist entries; nothing else depends on it until step 07.
- Step 02 moves the raw view. Reverting it restores `view.Raw` and needs steps 04 to 07 reverted first, because they use the shared `view.Styles`.
- Step 07 is the only step that changes what the user sees. Reverting it restores the single raw page. The steps before it stay valid: their functions go back to the allowlist.
