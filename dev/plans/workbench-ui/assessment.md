# Assessment

## Feasibility

High. Everything uses the standard library: `html/template`, `embed`, `encoding/json`, `os`, and `net/http` including `CrossOriginProtection`. No new module; `go.mod`, `go.sum` and `vendor/` do not change. The new code follows patterns the repository already has: handlers tested with `httptest`, a harness client of the inspected API, required flags without defaults, a supervisor that fails at startup on bad configuration.

Checked while writing the plan (probes in a scratch module, not in the repository):

- Toolchain `go1.27.1 windows/amd64`, `task` 3.53.1.
- `http.NewCrossOriginProtection().Handler`: POST without browser headers 204, POST with `Sec-Fetch-Site: same-origin` 204, POST with `Sec-Fetch-Site: cross-site` 403, GET with `Sec-Fetch-Site: cross-site` 204.
- `os.Rename` of a temporary file onto an existing file replaces it on Windows.
- `html/template`:
  - reads exported fields promoted through an unexported embedded struct, as `causeResource` embeds `explanationResource`;
  - renders `<script>x</script>` from data as `&lt;script&gt;x&lt;/script&gt;`;
  - supports a recursive `{{template "cause" .}}`;
  - replaces a `javascript:` URL with `#ZgotmplZ` and keeps `%2F` in hrefs;
  - writes `&` in attribute values as `&amp;` and `+` as `&#43;`. Link assertions therefore run on `html.UnescapeString(body)`.
- `url.Values.Encode` sorts keys: `kind`, `refresh`, `state`.
- `task clean` removes only `.test-results`, `.cache`, `site`, `bin` and `*.exe` files, so `.workbench-dashboards.json` in the repository root survives `task all`.
- `deadcode ./cmd/...` loads only packages reachable from `cmd`. `representation/dashboard` is invisible to it until step 04 imports it. `Create`, `Update` and `Delete` arrive in step 05 together with their callers, so no allowlist entry is needed.
- Inspected simulation: the clock runs from the start (`clock_running` true); `Advance` accepts 1 to `MaxAdvanceTicks`; error codes `invalid_dependency_mode` 422, `unknown_dependency` 404, `invalid_advance` 422, `unknown_product` 422.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| A template calls a method through reflection. `task deadcode` cannot see that call. | A method used only by a template is reported as dead, or removing it breaks a page only at runtime. | Rule in steps 03 to 06: templates read fields only. Pages are parsed at package init with `template.Must`; every page is executed by tests. |
| Inspected vocabulary slips into templates or UI text of the library. | `task boundary` fails. | Neutral wording in templates and fixtures; the scan runs in `task all` and covers `representation/templates` and `representation/dashboard`. |
| Panel removal by index from a stale page in a second tab. | The wrong panel is removed. | Documented in step 05. Single-user development tool; each `Update` is atomic. |
| Auto-refresh reloads a page while the user fills a form. | Typed input is lost. | Refresh is opt-in and off by default; the link to stop it is on every page. |
| The dashboards file is edited by hand while the workbench runs. | The next write overwrites the edit. | Documented in `representation/dashboard/doc.go`: one owner process. An invalid hand edit fails the next start with the file path and the reason. |
| The process stops during a write. | A partial temporary file. | Temporary file, `Sync`, `Rename`: the real file is either old or new. A leftover `.tmp` is truncated by the next write and ignored by git. |
| The operator and the inspector read the inspected service through the workbench listener. | During shutdown their requests fail. | Pages show 502 or 503; each read is bounded by its timeout. `Run` shutdown behavior is unchanged. |
| An entities panel shows the first `limit` matches in snapshot order; the inspected service lists orders oldest first. | A "failed orders" panel shows old failures first. | The panel shows the total and links to the full list. The library keeps the source order on purpose. |
| Step 04 is large (library and workbench in one step, forced by the `NewHandler` signature change). | Long step, late feedback. | Its tasks are ordered so that `go test ./representation/...` passes before the workbench part starts. |

## Dependencies

- No external module. The tools used by `task all` do not change.
- Live verification (`.todo`) needs a browser.
- Step order: 01 and 02 in any order; 03 after 02; 04 after 01 and 03; 05 after 04; 06 after 04; 07 after 05 and 06. Steps 05 and 06 are independent of each other.

## Validation

- Per step: the listed `go test` command, `go-arch-lint check`, `task boundary`, `task deadcode`, `task all`.
- Inspector stays read-only toward the source: `TestUIWritesDoNotObserve`; `git diff --stat` shows no change in `connectivity/`.
- Security: `TestUIEscapesSourceText`, `TestUIWritesRejectCrossSite`, `TestOperatorRejectsCrossSite`, `TestUIFormTooLarge`.
- Persistence: store tests reopen the file; `TestFailedWriteKeepsState`; `TestRunCreatesDashboardsFile`.
- End to end in process: `TestOperatorDrivesInspectorDashboard`.
- Live behavior: `.todo` section `## Workbench UI`, executed manually outside `task all`.

## Rollback

Nothing is committed by the implementer. To roll back the whole plan:

1. Delete `representation/dashboard/`, `representation/views.go`, `representation/html.go`, `representation/forms.go`, `representation/templates/`, `representation/html_test.go`, `representation/dashboard_view_test.go`, `representation/forms_test.go`, `harness/workbench/operator.go`, `harness/workbench/operator.html`, `harness/workbench/operator_test.go`, `.workbench-dashboards.json`.
2. `git restore representation harness/workbench cmd/workbench Taskfile.yml .gitignore .go-arch-lint.yml README.md harness/README.md .todo`.
3. `task all` passes with the previous state.

Rolling back a single step means reverting only the files that step lists under Target Artifacts, in reverse step order.
