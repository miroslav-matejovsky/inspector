---
title: "02 - Log file"
dependencies: []
effort: "S"
complexity: "low"
---

# 02 - Log file

## Objective

Package `internal/logfile` creates one log file per run in a directory and returns a `*slog.Logger` that writes to it in the `log/slog` text format (`key=value`). The directory is created when missing. An existing file is never overwritten.

## Target Artifacts

| File | Change |
| --- | --- |
| `internal/logfile/doc.go` | New: package documentation. |
| `internal/logfile/logfile.go` | New: `File`, `Open`, `Logger`, `Path`, `Close`. |
| `internal/logfile/logfile_test.go` | New: tests, package `logfile_test`. |
| `.go-arch-lint.yml` | Component `logfile: { in: internal/logfile }`. No `deps` entry: stdlib only. |
| `README.md` (root) | Row for `internal/logfile` in the `## Library` table. |

## Implementation Tasks

1. Write `logfile_test.go` with every test listed below.
2. Create `logfile.go`, then `doc.go`.
3. Add the component to `.go-arch-lint.yml`.
4. Add the row to the root `README.md`.
5. Run `go test ./internal/logfile/...` until it passes, then `task all`.

## Technical Details

### API (`logfile.go`)

```go
// File is the log file of one run and the logger that writes to it.
type File struct {
    file   *os.File
    logger *slog.Logger
}

// Open creates dir when it is missing and the file <name>-<time>.log in it,
// where <time> is now formatted as 20060102-150405 in the location of now.
// It fails when the file already exists. Records below level are dropped.
func Open(dir, name string, level slog.Level, now time.Time) (*File, error)

// Logger returns the logger that writes to the file.
func (f *File) Logger() *slog.Logger

// Path returns the path of the file.
func (f *File) Path() string

// Close flushes the file to disk and closes it.
func (f *File) Close() error
```

### `Open` sequence

1. `dir == ""`: return `logfile: dir is required`.
2. `name` does not match `^[a-z0-9][a-z0-9_-]{0,62}$`: return `logfile: name %q must match ^[a-z0-9][a-z0-9_-]{0,62}$`.
3. `os.MkdirAll(dir, 0o755)`.
4. `path := filepath.Join(dir, name + "-" + now.Format("20060102-150405") + ".log")`.
5. `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)`. `O_EXCL` fails on an existing file, so two runs started in the same second fail visibly instead of mixing their logs.
6. `slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: level}))`.
7. Errors are wrapped as `logfile: open %s: %w`, keeping the cause, so `errors.Is(err, fs.ErrExist)` works.

The handler writes each record with one `Write` call to the unbuffered `*os.File`. There is no buffer to flush and no goroutine.

`Close`: `f.file.Sync()` and `f.file.Close()`, joined with `errors.Join` and wrapped as `logfile: close %s: %w`.

### `doc.go`

1. First sentence: `Package logfile writes the runtime log of one run to its own file.`
2. File name format, directory creation, no overwrite.
3. Format: `log/slog` text handler; one record per line; `time`, `level`, `msg`, then attributes.
4. The caller owns the file and closes it after the last record.

### Root `README.md` row

```markdown
| `internal/logfile` | One log file per run, `log/slog` text format. | stdlib |
```

### Tests (`logfile_test.go`)

Fixed time: `time.Date(2026, 9, 30, 3, 52, 32, 0, time.UTC)`. Every test uses `t.TempDir()` and closes the file in `t.Cleanup` or before reading it.

| Test | Assertion |
| --- | --- |
| `TestOpenCreatesFile` | `Open(dir/logs, "app", slog.LevelInfo, fixed)`: `Path()` equals `filepath.Join(dir, "logs", "app-20260930-035232.log")`; the file exists although `dir/logs` did not. |
| `TestLoggerWritesToFile` | `Logger().Info("hello", "k", "v")`, `Close()`; the file content contains `level=INFO msg=hello k=v`. |
| `TestLevelFiltersRecords` | Level `slog.LevelWarn`: after `Info("quiet")` and `Warn("loud")` and `Close()`, the content contains `msg=loud` and not `msg=quiet`. |
| `TestOpenKeepsExistingFile` | Open, write `first`, Close. A second `Open` with the same arguments fails with `errors.Is(err, fs.ErrExist)`. The file still contains `msg=first`. |
| `TestOpenRejectsArguments` | Table: dir `""`; name `""`; name `App`; name `a/b`. Each `Open` fails; no file is created in the temporary directory. |

## Verification

```powershell
go test ./internal/logfile/...
go-arch-lint check
task all
```

## Acceptance Criteria

- `go test ./internal/logfile/...` exits 0 and runs every test listed above.
- `go list -deps ./internal/logfile` lists only standard library packages and `internal/logfile` itself.
- `.go-arch-lint.yml` maps `internal/logfile` to component `logfile` and has no `deps` entry for it.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task all` exits 0.

## Non-Goals

- Use by the workbench (step 04).
- Log rotation, size limits, deletion of old files.
- Writing to stderr and the file at the same time.
- JSON format.
