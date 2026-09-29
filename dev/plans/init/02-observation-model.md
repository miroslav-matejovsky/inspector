---
title: "02 - Observation model"
dependencies: ["01-library-boundary"]
effort: "S"
complexity: "low"
---

# 02 - Observation model

## Objective

Package `observation` holds the domain-neutral model of observed state: a `Snapshot` of entities and relations at one point in time. `NewSnapshot` is the only way to build a valid snapshot and rejects every broken invariant with an error that wraps `ErrInvalidSnapshot`. The package is pure: no goroutines, no I/O, no wall clock.

## Target Artifacts

| File | Change |
| --- | --- |
| `observation/doc.go` | New: package documentation (replaces `README.md`). |
| `observation/README.md` | Deleted. Its purpose and core questions move to `doc.go`. |
| `observation/snapshot.go` | New: `Ref`, `Attribute`, `Entity`, `Relation`, `Snapshot`, `NewSnapshot`, accessors, `ErrInvalidSnapshot`. |
| `observation/snapshot_test.go` | New tests, package `observation_test`. |
| `.go-arch-lint.yml` | Component `observation: { in: observation }`. No `deps` entry: stdlib only. |
| `README.md` (root) | Package table in section `## Library` with the `observation` row. |

## Implementation Tasks

1. Write `observation/snapshot_test.go` with every test listed below. Confirm it fails to compile.
2. Create `observation/snapshot.go`.
3. Create `observation/doc.go`; delete `observation/README.md`.
4. Add the component to `.go-arch-lint.yml`.
5. Add the package table to root `README.md` (Technical Details).
6. Run `go test ./observation/...` until it passes, then `task all`.

## Technical Details

### API (`snapshot.go`)

```go
// ErrInvalidSnapshot marks input that violates an invariant of NewSnapshot.
var ErrInvalidSnapshot = errors.New("invalid snapshot")

// Ref identifies one entity of an inspected system. Kind groups entities of
// the same type; ID is unique within its Kind. Both are chosen by the source
// and are opaque to Inspector.
type Ref struct {
    Kind string
    ID   string
}

// String returns "kind/id". It is meant for messages, not for parsing.
func (r Ref) String() string

// Attribute is one named fact about an entity, as reported by the source.
type Attribute struct {
    Name  string // not empty, unique within the entity
    Value string // may be empty
}

// Entity is one thing that exists in the inspected system.
type Entity struct {
    Ref        Ref
    State      string      // current state as named by the source; empty when the source reports none
    Attributes []Attribute // in source order; nil when there are none
}

// Relation is a directed, named connection between two entities.
type Relation struct {
    From Ref
    Kind string // name of the connection as chosen by the source, not empty
    To   Ref
}

// Snapshot is the state of one inspected system at one point in time.
// The zero value is an empty snapshot with a zero observation time.
type Snapshot struct {
    observedAt time.Time
    entities   []Entity
    relations  []Relation
    index      map[Ref]int // position of each entity in entities
}

// NewSnapshot validates its input and returns a snapshot. It keeps the given
// slices; the caller must not modify them afterwards.
func NewSnapshot(observedAt time.Time, entities []Entity, relations []Relation) (Snapshot, error)

// ObservedAt returns the time at which the source was observed.
func (s Snapshot) ObservedAt() time.Time

// Entities returns all entities in source order. The caller must not modify
// the returned slice.
func (s Snapshot) Entities() []Entity

// Relations returns all relations in source order. The caller must not modify
// the returned slice.
func (s Snapshot) Relations() []Relation

// Entity returns the entity with ref and whether it exists.
func (s Snapshot) Entity(ref Ref) (Entity, bool)
```

### Invariants of `NewSnapshot`

Checked in this order; the first violation is returned. Every error has the form `fmt.Errorf("observation: %w: <detail>", ErrInvalidSnapshot, ...)`. Indexes are zero-based positions in the input slices.

| Violation | Detail |
| --- | --- |
| `observedAt.IsZero()` | `observed time is zero` |
| entity `i` has empty `Ref.Kind` | `entity %d: empty kind` |
| entity `i` has empty `Ref.ID` | `entity %d: empty id` |
| entity `i` has the `Ref` of an earlier entity | `entity %d: duplicate %s` (`Ref.String()`) |
| attribute `j` of entity `ref` has empty `Name` | `entity %s: attribute %d: empty name` |
| two attributes of entity `ref` share a `Name` | `entity %s: duplicate attribute %q` |
| relation `k` has empty `Kind` | `relation %d: empty kind` |
| relation `k` has a `From` that is not an entity | `relation %d: unknown source entity %s` |
| relation `k` has a `To` that is not an entity | `relation %d: unknown target entity %s` |
| relation `k` equals an earlier relation (same `From`, `Kind`, `To`) | `relation %d: duplicate %s -%s-> %s` |

Allowed: empty `State`, empty attribute `Value`, a relation whose `From` equals its `To`, a snapshot without entities.

### `doc.go`

Content, in this order:

1. First sentence: `Package observation is the Observation domain of Inspector: it exposes the current state of an inspected system as facts, without interpretation.`
2. Core questions from the deleted `README.md`: What exists? What is its current state? What does the system currently know?
3. `# Model`: Snapshot, Entity (Ref, State, Attributes), Relation (From, Kind, To).
4. `# Vocabulary`: kinds, IDs, states, attribute names and relation kinds are strings chosen by the source. Inspector treats them as opaque and never hardcodes the vocabulary of an inspected system. A consumer maps its own system into this model.
5. `# Invariants`: the table above in prose, and that `NewSnapshot` is the only way to build a valid snapshot.
6. `# Ownership`: `NewSnapshot` keeps the given slices; accessors return stored slices; neither side may modify them.

### Root `README.md`

Append to section `## Library`:

```markdown
| Package | Domain | Responsibility | May depend on |
| --- | --- | --- | --- |
| `observation` | Observation | Domain-neutral model of observed state: entities, relations, snapshots. | stdlib |
```

### Tests (`snapshot_test.go`, package `observation_test`)

Fixture vocabulary is neutral (step 01 rejects the simulated domain). Shared fixture:

```go
var observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// b1 and b2 are books, a1 is their author; b1 cites b2.
b1 := observation.Entity{Ref: observation.Ref{Kind: "book", ID: "b1"}, State: "available",
    Attributes: []observation.Attribute{{Name: "title", Value: "Dune"}}}
b2 := observation.Entity{Ref: observation.Ref{Kind: "book", ID: "b2"}, State: "lent"}
a1 := observation.Entity{Ref: observation.Ref{Kind: "author", ID: "a1"},
    Attributes: []observation.Attribute{{Name: "name", Value: "Frank Herbert"}}}
relations := []observation.Relation{
    {From: b1.Ref, Kind: "written_by", To: a1.Ref},
    {From: b2.Ref, Kind: "written_by", To: a1.Ref},
    {From: b1.Ref, Kind: "cites", To: b2.Ref},
}
```

| Test | Assertion |
| --- | --- |
| `TestNewSnapshot` | Fixture is valid. `ObservedAt()` equals `observedAt`; `Entities()` equals `[b1, b2, a1]`; `Relations()` equals `relations`. |
| `TestNewSnapshotWithoutEntities` | `NewSnapshot(observedAt, nil, nil)` succeeds; `Entities()` and `Relations()` are empty. |
| `TestSnapshotEntity` | `Entity(b1.Ref)` returns `b1, true`; `Entity(Ref{"book", "zz"})` returns zero `Entity`, `false`. |
| `TestSnapshotEntityDistinguishesKinds` | Entities `book/x` and `author/x` are both accepted; `Entity` returns each by its own `Ref`. |
| `TestZeroSnapshot` | `observation.Snapshot{}.Entity(b1.Ref)` returns `false`; `Entities()` is empty. |
| `TestNewSnapshotAllowsSelfRelation` | Relation `b1 -cites-> b1` is accepted. |
| `TestNewSnapshotRejects` | Table, one case per row of the invariants table. Each case: `require.ErrorIs(err, observation.ErrInvalidSnapshot)` and `require.ErrorContains(err, <detail>)`, for example `entity 1: empty id`, `relation 0: unknown target entity author/zz`. |
| `TestRefString` | `Ref{Kind: "book", ID: "b1"}.String()` is `book/b1`. |

## Verification

```powershell
go test ./observation/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./observation/...` exits 0 and runs every test listed above.
- `observation/README.md` does not exist; `observation/doc.go` exists.
- `go list -deps ./observation` lists only standard library packages and `github.com/miroslav-matejovsky/inspector/observation`.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Typed attribute values. Values are strings.
- History of states or a diff between snapshots.
- Indexes for relations. Navigation scans relations (step 03).
- Deep copies of input or output slices.
