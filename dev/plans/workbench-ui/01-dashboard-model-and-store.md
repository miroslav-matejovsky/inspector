---
title: "01 - Dashboard model and file store"
dependencies: []
effort: "M"
complexity: "medium"
---

# 01 - Dashboard model and file store

## Objective

Package `representation/dashboard` defines what a dashboard is, validates it, and loads dashboards from one JSON file. `Open` creates the file when it is missing, so a wrong path or missing permission fails at startup. This step has the read side only; writes (`Create`, `Update`, `Delete`) come in step 05. The package uses only the standard library and knows no inspected system.

## Target Artifacts

| File | Change |
| --- | --- |
| `representation/dashboard/doc.go` | New: package documentation. |
| `representation/dashboard/dashboard.go` | New: `PanelType`, `Panel`, `Dashboard`, limits, `ErrInvalidDashboard`, `Validate`. |
| `representation/dashboard/store.go` | New: `FileStore`, `Open`, `List`, `Get`, unexported `save`. |
| `representation/dashboard/dashboard_test.go` | New: validation tests, package `dashboard_test`. |
| `representation/dashboard/store_test.go` | New: store tests, package `dashboard_test`. |
| `.go-arch-lint.yml` | Component `dashboard: { in: representation/dashboard }`. No `deps` entry: stdlib only. |
| `README.md` (root) | Row `representation/dashboard` in the `## Library` package table. |

## Implementation Tasks

1. Write `dashboard_test.go` and `store_test.go` with every test listed below. Confirm they fail to compile.
2. Create `dashboard.go`, then `store.go`.
3. Create `doc.go`.
4. Add the component to `.go-arch-lint.yml` (the package fails `go-arch-lint check` until it is mapped).
5. Add the row to the root `README.md`.
6. Run `go test -race ./representation/dashboard/...` until it passes, then `task all`.

## Technical Details

### Model (`dashboard.go`)

```go
// PanelType names what a panel shows.
type PanelType string

// Panel types.
const (
    PanelStates   PanelType = "states"   // entity counts by state for one kind
    PanelEntities PanelType = "entities" // entities of one kind, optionally in one state
    PanelEntity   PanelType = "entity"   // one entity with its state, reason and root causes
)

// Panel is one item of a dashboard. Which fields are set depends on Type,
// see Validate. Kind, State and ID are opaque strings of the inspected source.
type Panel struct {
    Type  PanelType `json:"type"`
    Kind  string    `json:"kind"`
    State string    `json:"state,omitempty"` // entities: optional state filter
    ID    string    `json:"id,omitempty"`    // entity: the entity ID
    Limit int       `json:"limit,omitempty"` // entities: how many entities are shown
}

// Dashboard is a named, ordered list of panels.
type Dashboard struct {
    ID     string  `json:"id"`
    Title  string  `json:"title"`
    Panels []Panel `json:"panels"`
}

// Limits of a valid dashboard.
const (
    MaxTitleRunes = 100
    MaxPanels     = 20
    MaxLimit      = 100
    MaxValueBytes = 256 // Kind, State and ID of a panel
)

// ErrInvalidDashboard marks a dashboard that violates a rule of Validate.
var ErrInvalidDashboard = errors.New("invalid dashboard")

// Validate returns the first violated rule as an error wrapping
// ErrInvalidDashboard, or nil.
func (d Dashboard) Validate() error
```

Rules, checked in this order:

| Field | Rule |
| --- | --- |
| `ID` | matches `^[a-z0-9][a-z0-9-]{0,62}$` |
| `Title` | `strings.TrimSpace(Title) != ""` and at most `MaxTitleRunes` runes |
| `Panels` | at most `MaxPanels`; zero panels is valid |
| `Panel.Type` | one of `states`, `entities`, `entity` |
| `Panel.Kind` | every type: not empty, at most `MaxValueBytes` bytes |
| `Panel.State` | `entities`: at most `MaxValueBytes` bytes, may be empty; other types: empty |
| `Panel.ID` | `entity`: not empty, at most `MaxValueBytes` bytes; other types: empty |
| `Panel.Limit` | `entities`: 1 to `MaxLimit`; other types: 0 |

Error format, like `observation`: `fmt.Errorf("dashboard: %w: %s", ErrInvalidDashboard, detail)`, for example `dashboard: invalid dashboard: panel 2: entities limit 0 must be from 1 to 100`. Panel errors name the panel index.

### Store (`store.go`)

```go
// FileStore keeps dashboards in one JSON file. It is safe for concurrent use.
// It holds all dashboards in memory; every write replaces the whole file
// atomically. One process owns the file: changes made by others while the
// store is open are overwritten by its next write.
type FileStore struct {
    path       string
    mu         sync.Mutex
    dashboards []Dashboard // file order
}

// Open loads the dashboards of the file at path. The directory of path must
// exist. When the file does not exist, Open creates it without dashboards.
func Open(path string) (*FileStore, error)

// List returns every dashboard in file order. The result is a copy.
func (s *FileStore) List() []Dashboard

// Get returns a copy of the dashboard with id and whether it exists.
func (s *FileStore) Get(id string) (Dashboard, bool)
```

File format, indented with two spaces, followed by a newline:

```json
{
  "dashboards": [
    {
      "id": "home",
      "title": "Home",
      "panels": [
        {"type": "states", "kind": "book"},
        {"type": "entities", "kind": "book", "state": "lent", "limit": 10},
        {"type": "entity", "kind": "book", "id": "b2"}
      ]
    }
  ]
}
```

`Open` sequence:

1. `path == ""` gives `dashboard: path is required`.
2. `os.Stat(filepath.Dir(path))` fails or is not a directory: `dashboard: open %s: directory %s: <cause>`.
3. File does not exist (`errors.Is(err, fs.ErrNotExist)`): `save(nil)`, return an empty store.
4. Read the file. Decode with `json.Decoder`, `DisallowUnknownFields`, then a second `Decode` must return `io.EOF` (trailing data is an error).
5. Validate every dashboard in file order; the first error is returned as `dashboard: open %s: dashboard %d: <err>`, still wrapping `ErrInvalidDashboard`.
6. A repeated `id` is `dashboard: open %s: dashboard %d: %w: duplicate id %q` wrapping `ErrInvalidDashboard`.

`save(dashboards []Dashboard) error` (unexported, called with `mu` held by writers in step 05):

1. Build the document; a nil list and every nil `Panels` are written as `[]`.
2. `json.MarshalIndent(doc, "", "  ")`, append `\n`.
3. Write to `path + ".tmp"` with `os.OpenFile(..., os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)`, `f.Sync()`, `f.Close()`.
4. `os.Rename(path+".tmp", path)`. On Windows it replaces the existing file (verified, see [assessment.md](assessment.md)).
5. On any error remove the temporary file and return the joined errors, prefixed `dashboard: save %s:`.

Copies: `List` and `Get` return dashboards whose `Panels` slices are clones (`slices.Clone`), so callers cannot change the store.

### `doc.go`

1. First sentence: `Package dashboard defines the dashboards of Inspector and keeps them in a JSON file.`
2. `# Model`: dashboard, panel types, the fields each type uses, the validation table.
3. `# File`: format, atomic replacement, creation of a missing file, strict decoding, single owner process.
4. `# Concurrency`: `FileStore` is safe for concurrent use; reads return copies.
5. Opaque strings: kinds, states and IDs are never interpreted.

### `.go-arch-lint.yml`

Under `components`: `dashboard: { in: representation/dashboard }`. Update the library comment to list `dashboard` among the library components.

### Root `README.md` row

```markdown
| `representation/dashboard` | Representation | Dashboard definitions: validation and a JSON file store. | stdlib |
```

### Tests

Fixture vocabulary: kinds `book`, `author`; states `available`, `lent`.

`dashboard_test.go`:

| Test | Assertion |
| --- | --- |
| `TestValidateAcceptsValidDashboards` | Table, each `NoError`: no panels; one panel of each type; entities without state; limits 1 and 100; title of 100 two-byte runes (`strings.Repeat(string(rune(0xE9)), 100)`); ID `"a" + strings.Repeat("0", 62)` (63 characters); 20 panels. |
| `TestValidateRejectsInvalidDashboards` | Table, each `ErrorIs(err, dashboard.ErrInvalidDashboard)`: ID `""`, `Home`, `-home`, `a_b`, `"a" + strings.Repeat("0", 63)`; title `""`, `"   "`, `strings.Repeat(string(rune(0xE9)), 101)`; 21 panels; type `""`, `chart`; states with state, with ID, with limit 1; entities with ID, limit 0, limit 101, empty kind; entity with empty ID, with state, with limit 1; kind of 257 bytes; entity ID of 257 bytes. |
| `TestValidateNamesPanel` | Third panel invalid: error message contains `panel 2`. |

`store_test.go` (every test uses `t.TempDir()`):

| Test | Assertion |
| --- | --- |
| `TestOpenCreatesMissingFile` | `Open(dir/d.json)` succeeds; `List()` is empty; file content `JSONEq` `{"dashboards": []}`. |
| `TestOpenLeavesNoTemporaryFile` | After `Open` created the file, `os.ReadDir(dir)` lists only `d.json`. |
| `TestOpenLoadsDashboards` | File with dashboards `a` and `b`: `List()` returns `a`, `b` with all panel fields; `Get("b")` is found; `Get("zz")` is not. |
| `TestOpenRejectsInvalidFile` | Table, each `Open` fails: `not json`; unknown field `{"dashboards": [], "x": 1}`; trailing data `{"dashboards": []} {}`; invalid dashboard (`ErrorIs ErrInvalidDashboard`); duplicate id (`ErrorIs ErrInvalidDashboard`, message contains `duplicate id`). |
| `TestOpenRequiresPathAndDirectory` | `Open("")` fails; `Open(dir/missing/d.json)` fails with a message containing `missing`. |
| `TestListAndGetReturnCopies` | Changing `List()[0].Panels[0].Kind` and `Get(id).Panels[0].Kind` does not change a later `List()`. |

## Verification

```powershell
go test -race ./representation/dashboard/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test -race ./representation/dashboard/...` exits 0 and runs every test listed above.
- `go list -deps ./representation/dashboard` lists only standard library packages and `representation/dashboard` itself.
- `.go-arch-lint.yml` maps `representation/dashboard` to component `dashboard` and has no `deps` entry for it.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- `Create`, `Update`, `Delete` (step 05).
- Use by `representation` or the workbench (step 04).
- File locking between processes, file versioning, migration.
