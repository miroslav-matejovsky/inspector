---
title: "05 - Workbench source and text view"
dependencies: ["03", "04"]
effort: "L"
complexity: "medium"
---

# 05 - Workbench source and text view

## Objective

The workbench runs a Source against the inspected simulation and shows what it collected:

- New required flags configure the Source. Targets are given as `-source-target name=path`, repeated; each path is read through the workbench listener.
- `Run` opens the Source after the listener and supervises three goroutines: the inspected app, the HTTP server and the Source. It closes the Source after they stop.
- The page at `/` shows one text table of the targets (signal count, time, status, duration, size, error of the latest signal) and a preview of each latest body. The page reloads every 2 seconds. This view is temporary.

## Target Artifacts

| File | Change |
| --- | --- |
| `harness/workbench/config.go` | `SourceConfig`, `SourceTarget`, `Config.Source`, `targetsFlag`, source flags, `sourceConfig`, validation. |
| `harness/workbench/workbench.go` | `Signals` interface; `Handler(inspected, signals, logger)` renders the page; `Run` opens, supervises and closes the Source. |
| `harness/workbench/signals.go` | New: `pageRefreshSeconds`, `previewBytes`, `writeSignals`. |
| `harness/workbench/index.html` | Becomes an `html/template` with the refresh meta tag and the `<pre id="signals">` block. |
| `harness/workbench/config_test.go` | Source flags, targets, invalid values. |
| `harness/workbench/workbench_test.go` | New `Handler` signature, page tests, `Run` tests with a database. |
| `harness/workbench/doc.go` | Flags, `# Source` and `# Page` sections, supervision of three goroutines. |
| `.go-arch-lint.yml` | `workbench: mayDependOn: [inspected, source]`. |
| `Taskfile.yml` | Source flags. |
| `.gitignore` | `/data/`. |
| `harness/README.md`, `README.md` | Flags, page, database file. |
| `.todo` | Section `## Source in the workbench`. |

## Implementation Tasks

1. Update `config_test.go` and `workbench_test.go` as listed below. Confirm they fail.
2. Update `config.go`.
3. Create `signals.go`. Turn `index.html` into a template.
4. Update `workbench.go`: `Signals`, `Handler`, `Run`.
5. Update `.go-arch-lint.yml`, `Taskfile.yml`, `.gitignore`.
6. Update `doc.go`, `harness/README.md`, `README.md`, `.todo`.
7. Run `go test ./harness/workbench/...` until it passes, then `task all`.

## Technical Details

### Configuration (`config.go`)

```go
// SourceConfig configures the Source of the workbench. Every field is required.
type SourceConfig struct {
    DatabasePath string        // SQLite file of the collected signals
    Interval     time.Duration // time between collection rounds
    Timeout      time.Duration // max wait for one read of one target
    Retention    time.Duration // signals older than this are deleted
    MaxBodyBytes int64         // a larger body is stored as a failed read
    Targets      []SourceTarget
}

// SourceTarget is one path of the workbench server that the Source reads.
type SourceTarget struct {
    Name string // source.Target.Name
    Path string // absolute path, for example /inspected/health/ready
}

type Config struct {
    Addr      string
    LogDir    string
    LogLevel  slog.Level
    Inspected inspected.Config
    Source    SourceConfig
}

// sourceConfig builds the source.Config that reads every target at baseURL + Path.
func (c SourceConfig) sourceConfig(baseURL string) source.Config
```

`Validate`, after the existing checks:

1. Every `Source.Targets[i].Path` starts with `/`. Otherwise: `workbench: source target %d: path %q must start with /`.
2. `c.Source.sourceConfig("http://" + c.Addr).Validate()`. This checks names, duplicates, durations, the body limit and the database path before anything starts. `Run` builds the real config from the listener address, because `Addr` may end in `:0`.

Flag type for the repeated target flag:

```go
// targetsFlag collects repeated -source-target name=path values in order.
type targetsFlag []SourceTarget

func (f *targetsFlag) String() string       // "name=path,name=path"; "" for a nil receiver
func (f *targetsFlag) Set(value string) error // strings.Cut(value, "="); no "=" gives `want name=path, got %q`
```

The `flag` package calls `String` on a zero value to print defaults, so a nil receiver must be handled.

New flags in `ParseConfig`:

| Flag | Binding | Usage text |
| --- | --- | --- |
| `-source-database` | `fs.StringVar(&cfg.Source.DatabasePath, ...)` | `SQLite file of the collected signals, for example data/source.db (required)` |
| `-source-interval` | `fs.DurationVar(&cfg.Source.Interval, ...)` | `time between collection rounds, for example 5s (required)` |
| `-source-timeout` | `fs.DurationVar(&cfg.Source.Timeout, ...)` | `max wait for one read of one target, for example 2s (required)` |
| `-source-retention` | `fs.DurationVar(&cfg.Source.Retention, ...)` | `signals older than this are deleted, for example 10m (required)` |
| `-source-max-body-bytes` | `fs.Int64Var(&cfg.Source.MaxBodyBytes, ...)` | `a larger body is stored as a failed read, for example 2097152 (required)` |
| `-source-target` | `fs.Var((*targetsFlag)(&cfg.Source.Targets), ...)` | `name=path of a workbench path to read, repeated (required)` |

### Page (`signals.go`, `index.html`)

```go
// pageRefreshSeconds is how often the page reloads itself.
const pageRefreshSeconds = 2

// previewBytes is the size of the body preview of each target.
const previewBytes = 512

// writeSignals writes the text view of summaries to w.
func writeSignals(w io.Writer, summaries []source.TargetSummary) error
```

`writeSignals` writes two parts.

1. A table through `tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)`, flushed before part 2:

   ```text
   TARGET        SIGNALS  OBSERVED              STATUS  DURATION  BYTES  ERROR
   health_ready  12       2026-09-30T01:02:03Z  503     1.5ms     246    -
   orders        0        -                     -       -         -      -
   ```

   | Column | Value with a latest signal | Without one |
   | --- | --- | --- |
   | `TARGET` | `Target.Name` | same |
   | `SIGNALS` | `Signals` | `0` |
   | `OBSERVED` | `Latest.ObservedAt.UTC().Format(time.RFC3339)` | `-` |
   | `STATUS` | `Latest.StatusCode`, `-` when 0 | `-` |
   | `DURATION` | `Latest.Duration.Round(time.Microsecond).String()` | `-` |
   | `BYTES` | `len(Latest.Body)` | `-` |
   | `ERROR` | `Latest.Error`, `-` when empty | `-` |

2. For each target in order with a non-empty `Latest.Body`: an empty line, then a header, then the preview on the next line:
   - header `--- <name>: <n> bytes ---` when `n <= previewBytes`;
   - header `--- <name>: first 512 of <n> bytes ---` otherwise;
   - preview `strings.ToValidUTF8(string(body[:min(n, previewBytes)]), "?")`.

`index.html` keeps its styles and the single panel. Changes:

- In `<head>`: `<meta http-equiv="refresh" content="{{.RefreshSeconds}}">`.
- In the panel: `<div class="content"><pre id="signals">{{.Signals}}</pre></div>`.
- Style: `pre { margin: 0; font: 12px/1.4 ui-monospace, monospace; }`.
- Parsed once: `var page = template.Must(template.New("index").Parse(indexHTML))`, with `indexHTML` a `string` from `//go:embed`. `html/template` escapes the text, so `<script>` in a body is shown as text.

### Handler (`workbench.go`)

```go
// Signals returns what the Source of the workbench collected, one summary per target.
type Signals interface {
    Summary(ctx context.Context) ([]source.TargetSummary, error)
}

// Handler serves the workbench page at "/" and delegates every path under
// inspected.PathPrefix+"/" to inspectedHandler, without stripping the prefix.
// The page shows the summaries of signals; a failed read of signals is
// logged to logger and shown on the page with status 500.
func Handler(inspectedHandler http.Handler, signals Signals, logger *slog.Logger) http.Handler
```

`GET /{$}`:

1. `summaries, err := signals.Summary(r.Context())`.
2. On error: `logger.Error("workbench: read signals", "error", err)`. The text becomes `signals unavailable: <err>` and the status 500.
3. Otherwise `writeSignals` into a `strings.Builder`. An error there is handled like step 2.
4. Execute `page` into a `bytes.Buffer` with `struct{ RefreshSeconds int; Signals string }`. On error, `http.Error(w, "render page", 500)` and log it. Otherwise set `Content-Type: text/html; charset=utf-8`, write the status and write the buffer. Keep the existing `_, _ = w.Write(...)`: an error while writing to the client has no recovery.

### `Run` (`workbench.go`)

Sequence:

1. `logger == nil` check, `cfg.Validate()`, `inspected.New`, `net.Listen`, all unchanged from step 04.
2. `src, err := source.Open(ctx, cfg.Source.sourceConfig("http://"+ln.Addr().String()), &http.Client{}, logger)`. On error, close the listener and return `errors.Join(fmt.Errorf("workbench: %w", err), <close error wrapped as workbench: close listener: %w>)`.
3. `logger.Info("workbench started", ...)`, unchanged.
4. Start three goroutines that report to one channel:

   ```go
   // exit is how one supervised goroutine stopped.
   type exit struct {
       name string // "inspected", "source" or "serve"
       err  error
   }

   exits := make(chan exit, 3)
   go func() { exits <- exit{"inspected", app.Run(runCtx)} }()
   go func() { exits <- exit{"source", src.Run(runCtx)} }()
   go func() { exits <- exit{"serve", srv.Serve(ln)} }()
   ```

   with `srv.Handler = Handler(app.Handler(), src, logger)`.

5. Wait for `ctx.Done()` or the first `exit`. For that first exit, `stopping := ctx.Err() != nil`, so a stop caused by cancellation that `select` picked first stays clean, as today.
6. `cancel()`, then `srv.Shutdown` with the existing 5 s timeout.
7. Receive the remaining exits (3 minus the one already received) with `stopping = true`.
8. `src.Close()`; add its error.
9. Log `workbench stopped` (step 04) and return `errors.Join(errs...)`.

Classification helper:

```go
// stopError returns nil when e is a clean stop and an error otherwise.
// stopping reports whether Run had asked the goroutine to stop.
func stopError(e exit, stopping bool) error {
    if stopping && (e.err == nil || errors.Is(e.err, http.ErrServerClosed)) {
        return nil
    }
    if e.err == nil {
        e.err = errors.New("unexpected stop")
    }
    return fmt.Errorf("workbench: %s stopped: %w", e.name, e.err)
}
```

It replaces the booleans `appExited` and `serveExited`. The behavior of the existing tests does not change.

### `.go-arch-lint.yml`

```yaml
  workbench:
    mayDependOn: [inspected, source]
```

### `Taskfile.yml`

Append to the folded `workbench` command of step 04:

```yaml
        -source-database data/source.db
        -source-interval 5s
        -source-timeout 2s
        -source-retention 10m
        -source-max-body-bytes 2097152
        -source-target index=/inspected/
        -source-target health_live=/inspected/health/live
        -source-target health_ready=/inspected/health/ready
        -source-target metrics=/inspected/metrics
        -source-target dependencies=/inspected/api/dependencies
        -source-target products=/inspected/api/products
        -source-target orders=/inspected/api/orders
```

Update the task `desc` to `Run the development workbench with the inspected simulation and its source on localhost:8080`.

### `.gitignore`

Append `/data/`.

### Documentation

- `harness/workbench/doc.go`:
  - Package summary: the page shows the signals collected by the Source.
  - All source flags in `# Configuration`.
  - New `# Source` section: the targets are paths on the workbench server; the Source reads them through the workbench listener like any client; the database file.
  - New `# Page` section: the table, the body preview, the reload every 2 seconds; temporary until the toolkit has a View.
  - `# Supervision` now names three goroutines.
- `harness/README.md`: the source flags in the flag table; a `Page` paragraph describing the text view; the database at `data/source.db`.
- `README.md`: the `## Harness` paragraph says that the workbench runs a Source against the simulation and shows the collected signals as text.
- `.todo`, new section:

  ```markdown
  ## Source in the workbench

  - Run `task workbench`, open http://localhost:8080/: the table lists the 7 targets. After 10 seconds every target has SIGNALS >= 2, STATUS 200 and ERROR `-`.
  - `curl -X PUT -d '{"mode":"outage"}' http://localhost:8080/inspected/sim/dependencies/payment-gateway`: within 10 seconds `health_ready` shows STATUS 503 and ERROR `-`. Set `healthy` again.
  - `data/source.db` exists. The log file has no `source target failing` line.
  - After 15 minutes, SIGNALS of every target stays at or below 121 (10m retention / 5s interval + 1).
  - Press Ctrl+C: the process exits with code 0 within 5 seconds.
  ```

### Tests

`config_test.go`:

| Test | Change or assertion |
| --- | --- |
| `allFlags` | Add `source-database` (a fixed value `data/source.db`), `source-interval 5s`, `source-timeout 2s`, `source-retention 10m`, `source-max-body-bytes 2097152`, `source-target alpha=/a`. |
| `TestParseConfig` | Expected `Source` field with all values and `Targets: []SourceTarget{{"alpha", "/a"}}`. |
| `TestParseConfigWithoutFlags` | The message lists every flag in `flag.VisitAll` order: `-addr, -inspected-orders-per-tick, -inspected-request-timeout, -inspected-seed, -inspected-tick-interval, -log-dir, -log-level, -source-database, -source-interval, -source-max-body-bytes, -source-retention, -source-target, -source-timeout`. |
| `TestParseConfigCollectsTargets` | New. `-source-target alpha=/a -source-target beta=/b` gives both targets in this order. |
| `TestParseConfigRejectsInvalidValues` | New cases: target without `=`, `alpha`; relative path, `alpha=a`; invalid name, `Alpha=/a`; zero interval, timeout and retention; body limit 0; empty database path. |
| `TestParseConfigRejectsDuplicateTargets` | New. `-source-target alpha=/a -source-target alpha=/b` fails with a message containing `duplicate name`. |

`workbench_test.go`:

| Test | Change or assertion |
| --- | --- |
| `validConfig(t)` | Takes `t`. `Source` has `DatabasePath` under `t.TempDir()`, `Interval` 1h, `Timeout` 1s, `Retention` 1h, `MaxBodyBytes` 1 MiB, one target `live=/inspected/health/live`. |
| `stubSignals` | New helper: returns fixed summaries or an error. Existing `Handler` tests pass `stubSignals{}` and `slog.New(slog.DiscardHandler)`. |
| `TestHandlerServesWorkbenchPanel` | Unchanged assertions, new signature. |
| `TestPageShowsSignals` | Stub: `alpha` with `Signals` 3 and a latest signal (t0 `2026-09-30T01:02:03Z`, status 200, duration 1500 microseconds, body `abc`); `beta` without one. The body matches the regexes `alpha\s+3\s+2026-09-30T01:02:03Z\s+200\s+1\.5ms\s+3\s+-` and `beta\s+0\s+-\s+-\s+-\s+-\s+-`, and contains `--- alpha: 3 bytes ---`. |
| `TestPageShowsFailedRead` | Stub latest with `StatusCode` 0 and `Error` `connection refused`: the row matches `alpha\s+1\s+\S+\s+-\s+\S+\s+0\s+connection refused`. |
| `TestPageTruncatesPreview` | Body of 600 `x`: contains `--- alpha: first 512 of 600 bytes ---` and 512 `x` in a row, but not 513. |
| `TestPageEscapesSignals` | Body `<script>alert(1)</script>`: the page contains `&lt;script&gt;` and does not contain `<script>alert`. |
| `TestPageRefreshes` | The page contains `<meta http-equiv="refresh" content="2">`. |
| `TestPageShowsSummaryError` | Stub returns `errors.New("boom")`: status 500; the body contains `signals unavailable: boom`; the log buffer contains `msg="workbench: read signals"`. |
| `TestPageShowsCollectedSignals` | Integration without `Run`: `startInspected(t)`; `httptest.NewServer(app.Handler())`; `source.Open` with targets `live` and `ready` at the server URL + `/inspected/health/live` and `/inspected/health/ready`, `srv.Client()`, a database in `t.TempDir()`; `src.Collect(ctx)`; then `GET /` through `Handler(app.Handler(), src, discard)`. The body matches `live\s+1\s+\S+\s+200` and `ready\s+1\s+\S+\s+200`. |
| `TestRunCreatesDatabase` | New. `ctx` cancelled before `Run`: `Run` returns nil, and the file at `cfg.Source.DatabasePath` exists. |
| `TestRunFailsWhenSourceCannotOpen` | New. `DatabasePath` is a file with the content `not a database`: `Run` returns an error containing `workbench: source:`. |
| existing `Run` tests | Use `validConfig(t)` and a discard logger; assertions unchanged. |

## Verification

```powershell
go test ./harness/workbench/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./harness/workbench/...` exits 0 and runs every test listed above.
- `task all` exits 0; its output contains `deadcode: no issues found` and `OK - No warnings found`.
- `Taskfile.yml` task `workbench` contains all 12 source flag lines listed above.
- `.gitignore` contains `/data/`.
- `.todo` contains `## Source in the workbench`.
- `git diff --stat` shows no change under `harness/inspected/`.

## Non-Goals

- A View package, HTML beyond the `<pre>` block, JavaScript, charts.
- Parsing or pretty-printing bodies.
- Controlling the Source from the page (pause, collect now).
- Reading `/inspected/sim`.
