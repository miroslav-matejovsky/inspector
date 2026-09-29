---
title: "03 - Navigation links"
dependencies: ["02-observation-model"]
effort: "S"
complexity: "low"
---

# 03 - Navigation links

## Objective

Package `navigation` answers "what is related to this entity and how" for one entity of an `observation.Snapshot`. `Links` returns every relation that touches the entity as a `Link` with a direction and the entity on the other end. The function is pure.

## Target Artifacts

| File | Change |
| --- | --- |
| `navigation/doc.go` | New: package documentation (replaces `README.md`). |
| `navigation/README.md` | Deleted. Its purpose and core questions move to `doc.go`. |
| `navigation/links.go` | New: `Direction`, `Outgoing`, `Incoming`, `Link`, `Links`, `ErrUnknownEntity`. |
| `navigation/links_test.go` | New tests, package `navigation_test`. |
| `.go-arch-lint.yml` | Component `navigation: { in: navigation }`; `deps`: `navigation: { mayDependOn: [observation] }`. |
| `README.md` (root) | Row `navigation` in the `## Library` package table. |

## Implementation Tasks

1. Write `navigation/links_test.go` with every test listed below. Confirm it fails to compile.
2. Create `navigation/links.go`.
3. Create `navigation/doc.go`; delete `navigation/README.md`.
4. Add the component and the rule to `.go-arch-lint.yml`.
5. Add the table row to root `README.md`.
6. Run `go test ./navigation/...` until it passes, then `task all`.

## Technical Details

### API (`links.go`)

```go
// ErrUnknownEntity marks a Ref that is not an entity of the snapshot.
var ErrUnknownEntity = errors.New("unknown entity")

// Direction tells on which end of a relation the starting entity is.
type Direction string

const (
    Outgoing Direction = "outgoing" // the starting entity is Relation.From
    Incoming Direction = "incoming" // the starting entity is Relation.To
)

// Link is one step from an entity to a related entity.
type Link struct {
    Relation  string          // observation.Relation.Kind
    Direction Direction
    Target    observation.Ref // the entity on the other end
}

// Links returns every link of the entity ref in s: first all outgoing links,
// then all incoming links, each group in the order of s.Relations(). A
// relation from an entity to itself yields one outgoing and one incoming link.
// An entity without relations has no links. Links returns an error wrapping
// ErrUnknownEntity when ref is not an entity of s.
func Links(s observation.Snapshot, ref observation.Ref) ([]Link, error)
```

Algorithm:

1. `if _, ok := s.Entity(ref); !ok` return `fmt.Errorf("navigation: %w: %s", ErrUnknownEntity, ref)`.
2. `links := []Link{}`.
3. For each relation `r` in `s.Relations()`: if `r.From == ref` append `Link{r.Kind, Outgoing, r.To}`.
4. For each relation `r` in `s.Relations()`: if `r.To == ref` append `Link{r.Kind, Incoming, r.From}`.
5. Return `links, nil`.

Linear in the number of relations. No index.

### `doc.go`

1. First sentence: `Package navigation is the Navigation domain of Inspector: it traverses the relations of an observed snapshot.`
2. Core questions from the deleted `README.md`: What is related? How is this connected? Where did this come from?
3. Direction semantics, link order, self relations, `ErrUnknownEntity`.

### Root `README.md` row

```markdown
| `navigation` | Navigation | Links of an entity in a snapshot, outgoing and incoming. | `observation` |
```

### Tests (`links_test.go`, package `navigation_test`)

Fixture: the same neutral snapshot as step 02 (`b1`, `b2` books, `a1` author; relations in this order: `b1 -written_by-> a1`, `b2 -written_by-> a1`, `b1 -cites-> b2`), built with `observation.NewSnapshot` in a test helper `fixture(t)`.

| Test | Assertion |
| --- | --- |
| `TestLinksOutgoingThenIncoming` | `Links(s, b1)` equals `[{written_by, outgoing, a1}, {cites, outgoing, b2}]`. `Links(s, b2)` equals `[{written_by, outgoing, a1}, {cites, incoming, b1}]`. `Links(s, a1)` equals `[{written_by, incoming, b1}, {written_by, incoming, b2}]`. |
| `TestLinksWithoutRelations` | Snapshot with one entity `book/lonely` and no relations: `Links` returns an empty slice and no error. |
| `TestLinksSelfRelation` | Snapshot with `book/x` and relation `x -cites-> x`: `Links` equals `[{cites, outgoing, x}, {cites, incoming, x}]`. |
| `TestLinksUnknownEntity` | `Links(s, Ref{"book", "zz"})` returns an error with `require.ErrorIs(err, navigation.ErrUnknownEntity)` and `require.ErrorContains(err, "book/zz")`. |

## Verification

```powershell
go test ./navigation/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./navigation/...` exits 0 and runs every test listed above.
- `navigation/README.md` does not exist; `navigation/doc.go` exists.
- `go list -deps ./navigation` lists only standard library packages and the packages `observation` and `navigation` of this module.
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Multi-step traversal, paths or graph search.
- Filtering links by relation kind or direction.
- Any HTTP or JSON (step 06).
