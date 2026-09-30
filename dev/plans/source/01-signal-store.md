---
title: "01 - Signal store and SQLite dependency"
dependencies: []
effort: "M"
complexity: "medium"
---

# 01 - Signal store and SQLite dependency

## Objective

Package `internal/signalstore` keeps signal records in one SQLite file. It:

- creates the file, its directory and the schema;
- inserts one round of records in one transaction;
- deletes records older than a given time;
- returns the record count and the latest record of every target.

The pure Go driver `github.com/ncruces/go-sqlite3` v0.35.6 is added to `go.mod` and vendored. Only this package imports it.

## Target Artifacts

| File | Change |
| --- | --- |
| `internal/signalstore/doc.go` | New: package documentation. |
| `internal/signalstore/store.go` | New: `Record`, `Stats`, `Store`, `Open`, `Insert`, `DeleteBefore`, `Stats`, `Close`. |
| `internal/signalstore/schema.go` | New: `schemaVersion`, DDL, unexported `migrate`. |
| `internal/signalstore/store_test.go` | New: tests, package `signalstore_test`. |
| `go.mod`, `go.sum`, `vendor/` | Add `github.com/ncruces/go-sqlite3 v0.35.6`. |
| `.go-arch-lint.yml` | Vendor `sqlite`, component `signalstore`, `deps` entry. |
| `README.md` (root) | New section `## Library` with the package table and the row for `internal/signalstore`. |

## Implementation Tasks

1. Write `internal/signalstore/store_test.go` with every test listed below.
2. Create `store.go` and `schema.go`. `store.go` imports the driver with `_ "github.com/ncruces/go-sqlite3/driver"`.
3. Run `go get github.com/ncruces/go-sqlite3@v0.35.6`, then `task vendor`. Confirm that `go list -m github.com/ncruces/go-sqlite3` prints `github.com/ncruces/go-sqlite3 v0.35.6`.
4. Create `doc.go`.
5. Update `.go-arch-lint.yml` (see below). Without the component, `go-arch-lint check` fails on the unmapped package.
6. Add the `## Library` section to the root `README.md`.
7. Run `go test ./internal/signalstore/...` until it passes, then `task all`.

## Technical Details

### API (`store.go`)

```go
// Record is one stored signal.
type Record struct {
    Target      string        // name of the target that was read
    URL         string        // URL that was read
    ObservedAt  time.Time     // start of the read; stored as Unix nanoseconds, read back in UTC
    Duration    time.Duration // time the read took
    StatusCode  int           // HTTP status; 0 when no response was received
    ContentType string        // Content-Type header of the response; empty when none
    Body        []byte        // response body; nil when none was stored
    Error       string        // why the read failed; empty when it succeeded
}

// Stats is what the store holds for one target.
type Stats struct {
    Target string
    Count  int64  // number of stored records of the target
    Latest Record // the record of the target that was inserted last
}

// Store keeps records in one SQLite file. It is safe for concurrent use:
// all operations share one connection and run one after another.
type Store struct {
    db *sql.DB
}

// Open opens the SQLite file at path. It creates the directory and the file
// when they are missing, and creates the schema in a new file.
func Open(ctx context.Context, path string) (*Store, error)

// Insert stores records in one transaction. An empty slice does nothing.
func (s *Store) Insert(ctx context.Context, records []Record) error

// DeleteBefore deletes every record with ObservedAt before t. It returns the
// number of deleted records. Records observed exactly at t are kept.
func (s *Store) DeleteBefore(ctx context.Context, t time.Time) (int64, error)

// Stats returns one entry per target that has records, ordered by target name.
func (s *Store) Stats(ctx context.Context) ([]Stats, error)

// Close closes the database.
func (s *Store) Close() error
```

### Schema (`schema.go`)

```go
// schemaVersion is stored in PRAGMA user_version.
const schemaVersion = 1
```

```sql
CREATE TABLE signals (
    id           INTEGER PRIMARY KEY,
    target       TEXT    NOT NULL,
    url          TEXT    NOT NULL,
    observed_at  INTEGER NOT NULL, -- Unix nanoseconds
    duration_ns  INTEGER NOT NULL,
    status_code  INTEGER NOT NULL,
    content_type TEXT    NOT NULL,
    body         BLOB,             -- NULL when no body was stored
    error        TEXT    NOT NULL
) STRICT;
CREATE INDEX signals_by_target ON signals (target, id);
CREATE INDEX signals_by_time ON signals (observed_at);
```

`migrate(ctx, db) error`:

1. Read `PRAGMA user_version` into an `int`.
2. `0`: in one transaction, run the DDL above, then `PRAGMA user_version = 1`, then commit.
3. `1`: return nil.
4. Any other value: return `schema version %d, want 1: delete the file to start empty`. This is the experimental phase: no migrations between versions.

### `Open` sequence

1. `path == ""`: return `signalstore: path is required`.
2. `strings.ContainsAny(path, "?#")`: return `signalstore: path %q must not contain '?' or '#'`. These characters would break the `file:` URI.
3. `os.MkdirAll(filepath.Dir(path), 0o755)`.
4. `sql.Open("sqlite3", "file:" + filepath.ToSlash(path) + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate")`.
5. `db.SetMaxOpenConns(1)`.
6. `migrate(ctx, db)`. A file that is not a database fails here with the driver error `file is not a database`.
7. On any error after step 4, close `db` and join both errors. Every error is wrapped as `signalstore: open %s: %w` with the path.

Why these pragmas:

- WAL lets a developer read the file with another SQLite client while the workbench writes to it. With the default rollback journal, that reader's lock would make `Insert` fail after the busy timeout, and the Source would stop the workbench.
- `busy_timeout(5000)` makes a locked write wait up to 5 s instead of failing at once.
- `_txlock=immediate` takes the write lock when `Insert` begins its transaction.

### `Insert`

1. `len(records) == 0`: return nil.
2. `tx, err := s.db.BeginTx(ctx, nil)`.
3. `stmt, err := tx.PrepareContext(ctx, "INSERT INTO signals (target, url, observed_at, duration_ns, status_code, content_type, body, error) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")`.
4. For each record, `stmt.ExecContext(ctx, r.Target, r.URL, r.ObservedAt.UnixNano(), int64(r.Duration), r.StatusCode, r.ContentType, r.Body, r.Error)`.
5. `stmt.Close()`, then `tx.Commit()`.
6. On any error, `errors.Join(err, tx.Rollback())`, wrapped as `signalstore: insert: %w`.

### `DeleteBefore`

`DELETE FROM signals WHERE observed_at < ?` with `t.UnixNano()`. Return `RowsAffected()`. Errors are wrapped as `signalstore: delete: %w`.

### `Stats`

```sql
SELECT s.target, c.n, s.url, s.observed_at, s.duration_ns, s.status_code, s.content_type, s.body, s.error
FROM (SELECT target, count(*) AS n, max(id) AS last_id FROM signals GROUP BY target) AS c
JOIN signals AS s ON s.id = c.last_id
ORDER BY s.target
```

- Scan `observed_at` and `duration_ns` into `int64`. Convert with `time.Unix(0, n).UTC()` and `time.Duration(d)`.
- Scan `body` into `[]byte`: NULL becomes nil. SQLite also returns an empty BLOB as nil. This is documented in `doc.go`.
- Check `rows.Err()`. Join the error of `rows.Close()` into the returned error through a named result and a deferred function; golangci-lint v2 `errcheck` rejects an unchecked `Close`.
- Wrap errors as `signalstore: stats: %w`.

`max(id)` is the last inserted record of a target. IDs only grow, because `DeleteBefore` removes old records, never the newest one.

### `Close`

`s.db.Close()`, wrapped as `signalstore: close: %w`.

### `doc.go`

1. First sentence: `Package signalstore keeps the signals collected by package source in an embedded SQLite database.`
2. `# File`: `file:` URI, WAL mode and its `-wal` and `-shm` files, busy timeout, directory created when missing, `?` and `#` rejected.
3. `# Schema`: the table, `PRAGMA user_version` = 1; any other version fails `Open`, and the fix is to delete the file.
4. `# Values`: times are Unix nanoseconds, read back in UTC; a nil or empty body reads back as nil.
5. `# Concurrency`: one connection; operations are serialized.
6. The driver is `github.com/ncruces/go-sqlite3`, pure Go, no cgo. Only this package imports it, so the database can be replaced here without touching `source`.

### `.go-arch-lint.yml`

```yaml
vendors:
  prometheus: { in: "github.com/prometheus/**" }
  sqlite: { in: "github.com/ncruces/go-sqlite3/**" }

components:
  # existing entries unchanged, plus:
  signalstore: { in: internal/signalstore }

deps:
  # existing entries unchanged, plus:
  signalstore:
    canUse: [sqlite]
```

### Root `README.md`

Add the section after the introduction:

```markdown
## Library

The toolkit follows the Alignment principle: Observe -> Source, Navigate -> Model, Explain -> View. Top-level packages are the public toolkit; `internal/` holds their implementation details. Toolkit packages never import `harness/...`; `go-arch-lint` enforces it.

| Package | Responsibility | May depend on |
| --- | --- | --- |
| `internal/signalstore` | SQLite storage of collected signals. | stdlib, `github.com/ncruces/go-sqlite3` |
```

### Tests (`store_test.go`)

Every test creates its database under `t.TempDir()`. It closes the store in `t.Cleanup`, registered after `t.TempDir()`, so the store closes before the directory is removed; Windows cannot remove an open file. Fixtures: targets `alpha` and `beta`, URLs `http://example.test/a` and `http://example.test/b`, times from `time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)`.

| Test | Assertion |
| --- | --- |
| `TestOpenCreatesDatabase` | `Open(ctx, dir/sub/s.db)` succeeds although `sub` does not exist; the file exists; `Stats` returns an empty slice. |
| `TestOpenReopensDatabase` | Insert 2 records, `Close`, `Open` the same path: `Stats` has count 2. |
| `TestOpenRejectsPath` | Table: `""`, `dir/a?b.db`, `dir/a#b.db`. Each `Open` fails; the message contains `signalstore`. |
| `TestOpenRejectsForeignFile` | A file with the content `not a database`: `Open` fails. |
| `TestOpenRejectsUnknownSchemaVersion` | Create the database with `Open` and `Close` it. Set `PRAGMA user_version = 2` through `database/sql` (the test imports `_ "github.com/ncruces/go-sqlite3/driver"`). `Open` fails with a message containing `schema version 2`. |
| `TestInsertAndStats` | Insert `alpha` #1, `beta` #1, `alpha` #2 with every field set. `Stats` returns `[{alpha 2 <alpha #2>}, {beta 1 <beta #1>}]`; `require.Equal` compares all fields. |
| `TestInsertKeepsFailedRead` | A record with `StatusCode 0`, `Body nil`, `Error "boom"`: the latest record has `Body` nil and `Error` `boom`. |
| `TestInsertNothing` | `Insert(ctx, nil)` returns nil; `Stats` is empty. |
| `TestDeleteBefore` | Records of `alpha` at t0, t0+1s, t0+2s. `DeleteBefore(t0+1s)` returns 1; `Stats` has count 2 and the latest record is the one at t0+2s. |
| `TestStoreFailsAfterClose` | After `Close`, `Insert` returns an error. |

## Verification

```powershell
go list -m github.com/ncruces/go-sqlite3
go test ./internal/signalstore/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go list -m github.com/ncruces/go-sqlite3` prints `github.com/ncruces/go-sqlite3 v0.35.6`.
- `vendor/modules.txt` contains `# github.com/ncruces/go-sqlite3 v0.35.6`.
- `go test ./internal/signalstore/...` exits 0 and runs every test listed above.
- `go list -deps ./internal/signalstore` prints no path containing `/harness/`.
- `go list -f "{{.ImportPath}} {{.Imports}} {{.XTestImports}}" ./... | Select-String ncruces` prints only lines that start with `github.com/miroslav-matejovsky/inspector/internal/signalstore`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Use by `source` (step 03).
- Queries other than per-target stats: history of a target, time ranges, body search.
- Schema migrations between versions.
- Storing unchanged bodies once, compression.
