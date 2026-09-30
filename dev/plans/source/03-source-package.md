---
title: "03 - Source package"
dependencies: ["01"]
effort: "L"
complexity: "medium"
---

# 03 - Source package

## Objective

Package `source` is the Observe part of the toolkit. A `Source`:

- reads every configured target with HTTP GET in one round (`Collect`) and stores each response or failed read as a signal in `internal/signalstore`;
- deletes signals older than the retention after each round;
- repeats rounds on an interval until its context ends (`Run`);
- reports per target how many signals are stored and which is the latest (`Summary`);
- logs when a target starts failing and when it recovers.

The package knows no inspected system. It does not parse bodies.

## Target Artifacts

| File | Change |
| --- | --- |
| `source/doc.go` | New: package documentation. |
| `source/config.go` | New: `Target`, `Config`, `Validate`. |
| `source/source.go` | New: `Signal`, `TargetSummary`, `Source`, `Open`, `Collect`, `Run`, `Summary`, `Close`, unexported `read`, `logTransitions`, `toRecord`, `fromRecord`. |
| `source/export_test.go` | New: `SetClock`, compiled only in tests. |
| `source/config_test.go` | New: validation tests, package `source_test`. |
| `source/source_test.go` | New: behavior tests, package `source_test`. |
| `.go-arch-lint.yml` | Component `source: { in: source }`; `deps` entry `source: mayDependOn: [signalstore]`. |
| `README.md` (root) | Row for `source` in the `## Library` table. |

## Implementation Tasks

1. Write `config_test.go`, `source_test.go` and `export_test.go` with every test listed below.
2. Create `config.go`, then `source.go`.
3. Create `doc.go`.
4. Update `.go-arch-lint.yml`.
5. Add the row to the root `README.md`, above the `internal/` rows.
6. Run `go test ./source/...` until it passes, then `task all`.

## Technical Details

### Configuration (`config.go`)

```go
// Target is one HTTP endpoint that a Source reads with GET.
type Target struct {
    Name string // unique name of the target, see Validate
    URL  string // absolute http or https URL
}

// Config configures a Source. Every field is required and has no default.
type Config struct {
    Targets      []Target      // read in every round, at least one
    Interval     time.Duration // time between the starts of two rounds
    Timeout      time.Duration // max wait for one read of one target
    Retention    time.Duration // signals older than this are deleted after each round
    MaxBodyBytes int64         // a larger body is stored as a failed read
    DatabasePath string        // SQLite file, see package internal/signalstore
}

// Validate reports the first rule that c breaks, or nil.
func (c Config) Validate() error
```

Rules, checked in this order:

| Field | Rule | Error |
| --- | --- | --- |
| `Targets` | `len >= 1` | `source: at least one target is required` |
| `Target.Name` | matches `^[a-z0-9][a-z0-9_-]{0,62}$` | `source: target %d: name %q must match ^[a-z0-9][a-z0-9_-]{0,62}$` |
| `Target.Name` | unique | `source: target %d: duplicate name %q` |
| `Target.URL` | `url.Parse` succeeds, scheme is `http` or `https`, host is not empty | `source: target %d: url %q must be an absolute http or https URL` |
| `Interval` | `> 0` | `source: interval must be positive, got %s` |
| `Timeout` | `> 0` | `source: timeout must be positive, got %s` |
| `Retention` | `> 0` | `source: retention must be positive, got %s` |
| `MaxBodyBytes` | `> 0` | `source: max body bytes must be positive, got %d` |
| `DatabasePath` | not empty | `source: database path is required` |

The target index in errors starts at 0.

### Types and constructor (`source.go`)

```go
// Signal is one read of one target.
type Signal struct {
    Target      string        // Target.Name
    URL         string        // Target.URL
    ObservedAt  time.Time     // start of the read, UTC
    Duration    time.Duration // from the start of the read until the body was read or the read failed
    StatusCode  int           // HTTP status; 0 when no response was received
    ContentType string        // Content-Type header; empty when no response was received
    Body        []byte        // response body; nil when Error is set
    Error       string        // why the read failed; empty when a body within MaxBodyBytes was read
}

// TargetSummary is what is stored for one configured target.
type TargetSummary struct {
    Target  Target
    Signals int64   // number of stored signals of the target
    Latest  *Signal // the signal stored last; nil when none is stored
}

// Source collects signals from the targets of its Config.
type Source struct {
    cfg     Config
    client  *http.Client
    logger  *slog.Logger
    store   *signalstore.Store
    now     func() time.Time // time.Now; replaced in tests by SetClock
    mu      sync.Mutex       // serializes rounds
    failing map[string]bool  // target name -> the last read failed; guarded by mu
}

// Open validates cfg and opens the database at cfg.DatabasePath. client
// sends the requests; each request is bounded by cfg.Timeout, so client
// needs no timeout of its own. logger receives start and target failure and
// recovery records. Close releases the database.
func Open(ctx context.Context, cfg Config, client *http.Client, logger *slog.Logger) (*Source, error)
```

`Open` sequence: `cfg.Validate()`; `client == nil` gives `source: http client is required`; `logger == nil` gives `source: logger is required`; `signalstore.Open(ctx, cfg.DatabasePath)` wrapped as `source: %w`. It returns `&Source{..., now: time.Now, failing: map[string]bool{}}`. It checks everything before it opens the database, so an invalid config creates no file.

### `Collect`

```go
// Collect reads every target once, concurrently, and stores one signal per
// target in one transaction. Then it deletes signals observed before the
// start of the round minus Retention. A failed read is a stored signal, not
// an error. Collect returns an error only when the store fails. Rounds do not
// overlap: a second call waits for the first.
func (s *Source) Collect(ctx context.Context) error
```

1. `s.mu.Lock()`, `defer s.mu.Unlock()`.
2. `start := s.now()`.
3. `signals := make([]Signal, len(s.cfg.Targets))`. For each target `i, t`, `wg.Go(func() { signals[i] = s.read(ctx, t) })` with a `sync.WaitGroup`. Then `wg.Wait()`. Signals keep the order of the targets.
4. `s.logTransitions(signals)`.
5. `s.store.Insert(ctx, records)`, where `records` is `toRecord` of each signal. Wrap the error as `source: %w`.
6. `s.store.DeleteBefore(ctx, start.Add(-s.cfg.Retention))`. Wrap the error as `source: %w`.

`read(ctx context.Context, t Target) Signal`:

1. `start := s.now()`. `sig := Signal{Target: t.Name, URL: t.URL, ObservedAt: start.UTC()}`.
2. `ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)`, `defer cancel()`.
3. `req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)`. On error, set `sig.Error = err.Error()`.
4. `resp, err := s.client.Do(req)`. On error, set `sig.Error = err.Error()`.
5. Otherwise set `sig.StatusCode = resp.StatusCode` and `sig.ContentType = resp.Header.Get("Content-Type")`. Read `body, readErr := io.ReadAll(io.LimitReader(resp.Body, s.cfg.MaxBodyBytes+1))`, then `closeErr := resp.Body.Close()`.
6. The first match sets the outcome:
   - `readErr != nil`: `sig.Error = "read body: " + readErr.Error()`
   - `int64(len(body)) > s.cfg.MaxBodyBytes`: `sig.Error = fmt.Sprintf("body exceeds %d bytes", s.cfg.MaxBodyBytes)`
   - `closeErr != nil`: `sig.Error = "close body: " + closeErr.Error()`
   - otherwise: `sig.Body = body`
7. `sig.Duration = s.now().Sub(start)` on every path. Return `sig`.

A response with any status code, including 4xx and 5xx, is a successful read: the status is part of the signal. Only a missing response, an unreadable body or a body that is too large sets `Error`.

`logTransitions(signals []Signal)`, called with `mu` held. For each signal, `failing := sig.Error != ""`. When `failing == s.failing[sig.Target]`, skip it. Otherwise store the new value and log:

- start failing: `s.logger.Warn("source target failing", "target", sig.Target, "url", sig.URL, "error", sig.Error)`
- recover: `s.logger.Info("source target recovered", "target", sig.Target, "url", sig.URL)`

A target that has not been read yet counts as not failing, so its first successful read logs nothing.

### `Run`

```go
// Run collects one round at once and then one round per Interval until ctx
// ends. Ticks that arrive during a round are dropped (time.Ticker). Run
// returns nil when ctx ends and the error of Collect when the store fails.
func (s *Source) Run(ctx context.Context) error
```

1. `s.logger.Info("source started", "targets", len(s.cfg.Targets), "interval", s.cfg.Interval, "database", s.cfg.DatabasePath)`.
2. `ticker := time.NewTicker(s.cfg.Interval)`, `defer ticker.Stop()`.
3. Loop:
   - `err := s.Collect(ctx)`. If `err != nil` and `ctx.Err() != nil`, return nil: the round was interrupted by the end of `ctx`. If `err != nil`, return `err`.
   - `select { case <-ctx.Done(): return nil; case <-ticker.C: }`.

### `Summary` and `Close`

```go
// Summary returns one entry per configured target, in the order of
// Config.Targets. Signals stored under names that are not configured, for
// example by an earlier run with other targets, are left out.
func (s *Source) Summary(ctx context.Context) ([]TargetSummary, error)

// Close closes the database. Call it after Run has returned.
func (s *Source) Close() error
```

`Summary` calls `s.store.Stats(ctx)` (errors wrapped as `source: %w`) and indexes the result by target name. For each configured target it sets `Signals` and `Latest` (`fromRecord`) when stats exist; otherwise `Signals` is 0 and `Latest` is nil. `Close` returns `s.store.Close()` wrapped as `source: %w`.

`toRecord` and `fromRecord` copy the eight fields one to one between `Signal` and `signalstore.Record`. The two types stay separate: `source` is public API, `signalstore` is replaceable storage.

### `export_test.go`

```go
package source

// SetClock replaces the clock of s. Only tests use it.
func SetClock(s *Source, now func() time.Time) { s.now = now }
```

### `doc.go`

1. First sentence: `Package source collects observable signals from a running system and stores them.`
2. Its place in the toolkit: Observe -> Source. It takes what the system exposes through HTTP GET, raw and unparsed. The Model interprets signals later.
3. `# Signals`: one per target and round; the fields; a status code of any value is a successful read; what counts as a failed read.
4. `# Rounds`: concurrent reads, one transaction, retention after each round, rounds never overlap, ticks during a round are dropped.
5. `# Storage`: SQLite through `internal/signalstore`, one file at `Config.DatabasePath`.
6. `# Logging`: `source started`; one `source target failing` record when a target starts failing and one `source target recovered` when it recovers.
7. `# Lifecycle`: `Open`, then `Run` in a goroutine owned by the caller, then `Close` after `Run` returned. `Summary` is safe to call concurrently with `Run`.

### `.go-arch-lint.yml`

```yaml
components:
  source: { in: source }

deps:
  source:
    mayDependOn: [signalstore]
```

### Root `README.md` row

```markdown
| `source` | Observe: collects signals from HTTP targets on an interval and stores them. | `internal/signalstore` |
```

### Tests

Helpers in `source_test.go`:

- `validConfig(t)`: targets `alpha -> <srv>/a`, `beta -> <srv>/b`, `Interval` 1h, `Timeout` 1s, `Retention` 1h, `MaxBodyBytes` 1024, `DatabasePath` under `t.TempDir()`.
- `open(t, cfg, logger)`: `source.Open` with `http.DefaultClient`, `SetClock` to a clock the test controls, and `Close` in `t.Cleanup`.
- Test server: `httptest.NewServer` with `/a` answering 200, `Content-Type: text/plain; charset=utf-8`, body `alpha body`; `/b` answering 503, `Content-Type: application/json`, body `{"s":"down"}`.
- Clock: `t0 := time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)`; the fake clock returns a variable that the test sets between calls.
- Logger for log assertions: `slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{ReplaceAttr: dropTime}))`, where `dropTime` removes `slog.TimeKey`.

`config_test.go`:

| Test | Assertion |
| --- | --- |
| `TestConfigValidateAccepts` | Table, each `NoError`: the valid config; a name of 63 characters; an `https` URL; names with `_` and `-`. |
| `TestConfigValidateRejects` | Table, each `Error`: no targets; name `""`, `Alpha`, `-a`, 64 characters; duplicate name; URL `""`, `ftp://x/a`, `http:///a`, `/a`, `::`; `Interval`, `Timeout`, `Retention` each 0 and -1s; `MaxBodyBytes` 0; `DatabasePath` `""`. |
| `TestConfigValidateNamesTarget` | Second target invalid: the message contains `target 1`. |

`source_test.go`:

| Test | Assertion |
| --- | --- |
| `TestOpenRejectsArguments` | Invalid config, nil client, nil logger: each `Open` fails, and no file exists at `DatabasePath`. |
| `TestSummaryBeforeCollect` | Targets `beta`, `alpha` in this order: `Summary` returns `beta`, `alpha`, each with `Signals` 0 and `Latest` nil. |
| `TestCollectStoresSignals` | Clock at t0. After `Collect`, `Summary`: `alpha` has `Signals` 1 and `Latest` equal to `{Target: "alpha", URL: <srv>/a, ObservedAt: t0, Duration: 0, StatusCode: 200, ContentType: "text/plain; charset=utf-8", Body: []byte("alpha body"), Error: ""}`. `beta` has `StatusCode` 503, `Body` `{"s":"down"}` and an empty `Error`. |
| `TestCollectUsesGet` | The handler of `/a` records method and body length: `GET` and 0. |
| `TestCollectStoresFailedRead` | Target URL of a server that was closed before `Collect`: `Collect` returns nil; `Latest` has `StatusCode` 0, `Body` nil, `Error` not empty. |
| `TestCollectRejectsLargeBody` | `MaxBodyBytes` 10; `/ten` answers 10 bytes, `/eleven` answers 11 bytes. `ten` has `Body` of 10 bytes and an empty `Error`; `eleven` has `StatusCode` 200, `Body` nil, `Error` `body exceeds 10 bytes`. |
| `TestCollectTimesOut` | `Timeout` 50ms; `/slow` waits for `<-r.Context().Done()`. `Error` contains `context deadline exceeded`. |
| `TestCollectAppliesRetention` | `Retention` 1m. Collect at t0, t0+30s and t0+90s. `alpha` has `Signals` 2, and `Latest.ObservedAt` is t0+90s. |
| `TestCollectLogsTransitionsOnce` | `MaxBodyBytes` 1; `/toggle` answers the body `xx` (a failed read) or an empty body (a successful read), switched by an `atomic.Bool`. Rounds: fail, fail, ok, ok. The log contains `msg="source target failing"` once and `msg="source target recovered"` once, the failing line before the recovered line. |
| `TestSummaryIgnoresUnknownTargets` | Source with target `alpha` collects once and is closed. A second Source on the same database with target `beta` only: `Summary` returns only `beta`, with `Signals` 0. |
| `TestRunStopsWhenContextEnds` | `ctx` cancelled before `Run`: `Run` returns nil. |
| `TestRunReturnsStoreError` | `Close` the source, then `Run` with a live `ctx`: `Run` returns an error whose message starts with `source:`; the log contains `msg="source started"`. `t.Cleanup` must not call `Close` twice; this test opens its Source without the `open` helper. |

The `Run` loop is not tested with real ticks: that would need sleeps. `Collect` covers what a round does. The two `Run` tests cover how it starts and stops.

## Verification

```powershell
go test ./source/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./source/...` exits 0 and runs every test listed above.
- `go list -deps ./source` prints no path containing `/harness/`.
- `go vet ./source/...` exits 0.
- `.go-arch-lint.yml` maps `source` to component `source` with `mayDependOn: [signalstore]`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Use by the workbench (step 05).
- Parsing bodies, request headers other than the defaults of `net/http`, authentication.
- Reading the history of a target; the only query is `Summary`.
- Retries within a round.
- Per-target intervals or timeouts.
