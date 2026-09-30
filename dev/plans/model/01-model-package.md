---
title: "01 - Model package"
dependencies: []
effort: "M"
complexity: "medium"
---

# 01 - Model package

## Objective

The top-level package `model` exists:

- `model.Build` turns the latest signals of a `source.Source` into a read-only `*model.Model`. The model holds entities (kind, name, state, properties, links, evidence), the backlinks computed from the links, and an issue for everything that could not be modeled.
- `model.New` assembles and validates a model from entities that are already interpreted.

Only tests call the package in this step. Its functions are staged in the deadcode allowlist until step 07.

## Target Artifacts

| File | Change |
| --- | --- |
| `model/doc.go` | new: package documentation |
| `model/model.go` | new: types, `New`, `validate`, read methods of `*Model` |
| `model/build.go` | new: `Interpreter`, `Build` |
| `model/model_test.go` | new: tests of `New` and the read methods (package `model_test`) |
| `model/build_test.go` | new: tests of `Build` (package `model_test`) |
| `.go-arch-lint.yml` | component `model`, may depend on `source` |
| `taskfile/deadcode.ps1` | staged names of the unreachable functions of `model` |
| `README.md` | Library table: row `model` |

## Implementation Tasks

1. Write `model/model_test.go` and `model/build_test.go` with the tests listed under Verification. At this point they do not compile.
2. Create `model/model.go` with the code of "Types" and "New and read methods" below.
3. Create `model/build.go` with the code of "Build" below.
4. Run `go test ./model/...` and fix the code until every test passes.
5. Create `model/doc.go` with the text of "Package documentation" below.
6. In `.go-arch-lint.yml`, add `model: { in: model }` under `components`, after `source`, and `model: { mayDependOn: [source] }` under `deps`, after `source`.
7. Run `deadcode ./cmd/...`. For every line it prints for a file under `model/`, copy the name after `unreachable func: ` into `$allow` in `taskfile/deadcode.ps1`: one double-quoted string per line, under the comment line `# staged by dev/plans/model, removed in step 07`.
8. Add the `model` row to the Library table of the root `README.md`, after the `source` row: `` | `model` | Navigate: builds a read-only graph of entities with state, properties and links from the latest signals of a Source, through interpreters. | `source` | ``.
9. Run `task all`.

## Technical Details

### Types

`model/model.go`:

```go
package model

// Health is how well an entity does, as its signal states it. The set is
// closed: views color by it.
type Health string

// Health values.
const (
	HealthUnknown  Health = "unknown" // the signal states no health
	HealthOK       Health = "ok"
	HealthDegraded Health = "degraded"
	HealthDown     Health = "down"
)

// State is the condition of an entity in the words of the inspected system.
type State struct {
	Value  string // for example "outage" or "pending"; empty when the signal states none
	Health Health // empty is read as HealthUnknown by New
	Reason string // why the entity is in this state; empty when the signal states none
}

// Property is one named fact about an entity, as its signal states it.
type Property struct {
	Name  string
	Value string
}

// Link is a relation from the entity that holds it to the entity with the ID To.
type Link struct {
	Type string // what the relation means, read from the holder to To, for example "waits on"
	To   string // ID of the related entity; it may be missing from the model
}

// Backlink is a Link seen from the entity it points to.
type Backlink struct {
	From string // ID of the entity that holds the Link
	Type string // Link.Type
}

// Evidence is the signal an entity was read from.
type Evidence struct {
	Target     string    // name of the target
	URL        string    // URL of the target
	ObservedAt time.Time // start of the read, UTC
}

// Entity is one thing of the inspected system.
type Entity struct {
	ID         string     // unique in a Model and the same in every build
	Kind       string     // what the entity is, for example "order"; interpreters define kinds
	Name       string     // short name for humans; New sets an empty Name to ID
	State      State
	Properties []Property // in the order the interpreter gives them
	Links      []Link     // in the order the interpreter gives them
	Evidence   Evidence   // set by Build
}

// Issue is a signal, or an entity read from it, that could not be modeled.
type Issue struct {
	Target string // name of the target of the signal
	Error  string
}

// Model is a read-only snapshot of the inspected system. The slices returned
// by its methods must not be modified.
type Model struct {
	entities  []Entity              // in the order of New
	byID      map[string]int        // Entity.ID -> index in entities
	backlinks map[string][]Backlink // Link.To -> links to it, in entity and link order
	issues    []Issue               // given issues first, then those found by New
}
```

### New and read methods

`model/model.go`, continued:

```go
// New returns the model of entities, in their order. An entity that is not
// valid, see validate, or whose ID an earlier entity has, is left out and
// reported as an Issue of its Evidence.Target, after the given issues. New
// sets an empty State.Health to HealthUnknown and an empty Name to ID. Links
// to IDs that no entity has are kept.
func New(entities []Entity, issues []Issue) *Model {
	m := &Model{
		byID:      map[string]int{},
		backlinks: map[string][]Backlink{},
		issues:    slices.Clone(issues),
	}
	for _, e := range entities {
		if err := validate(e); err != nil {
			m.issues = append(m.issues, Issue{Target: e.Evidence.Target, Error: err.Error()})
			continue
		}
		if i, ok := m.byID[e.ID]; ok {
			m.issues = append(m.issues, Issue{
				Target: e.Evidence.Target,
				Error:  fmt.Sprintf("entity %q: already read from target %q", e.ID, m.entities[i].Evidence.Target),
			})
			continue
		}
		if e.State.Health == "" {
			e.State.Health = HealthUnknown
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		m.byID[e.ID] = len(m.entities)
		m.entities = append(m.entities, e)
	}
	for _, e := range m.entities {
		for _, l := range e.Links {
			m.backlinks[l.To] = append(m.backlinks[l.To], Backlink{From: e.ID, Type: l.Type})
		}
	}
	return m
}

// validate returns why e cannot be part of a model, or nil.
func validate(e Entity) error {
	switch {
	case e.ID == "":
		return fmt.Errorf("entity of kind %q named %q: empty id", e.Kind, e.Name)
	case e.Kind == "":
		return fmt.Errorf("entity %q: empty kind", e.ID)
	}
	switch e.State.Health {
	case "", HealthUnknown, HealthOK, HealthDegraded, HealthDown:
	default:
		return fmt.Errorf("entity %q: unknown health %q", e.ID, e.State.Health)
	}
	for i, l := range e.Links {
		if l.Type == "" || l.To == "" {
			return fmt.Errorf("entity %q: link %d: empty type or target", e.ID, i)
		}
	}
	return nil
}

// Entities returns every entity of the model, in the order of New.
func (m *Model) Entities() []Entity { return m.entities }

// Entity returns the entity with the ID id; ok is false when there is none.
func (m *Model) Entity(id string) (e Entity, ok bool) {
	i, ok := m.byID[id]
	if !ok {
		return Entity{}, false
	}
	return m.entities[i], true
}

// Backlinks returns the links to id, in entity and link order. id does not
// have to be an entity of the model.
func (m *Model) Backlinks(id string) []Backlink { return m.backlinks[id] }

// Issues returns what could not be modeled: the issues given to New first,
// then the entities that New left out, in their order.
func (m *Model) Issues() []Issue { return m.issues }
```

### Build

`model/build.go`:

```go
package model

// Interpreter reads the entities that one signal describes. It gets only
// signals that were read (Signal.Error is empty), with any status code, and
// must not modify the body. An error drops every entity of the signal.
type Interpreter func(sig source.Signal) ([]Entity, error)

// Build builds the model of the latest signal of every summary, in the order
// of summaries. interpreterOf returns the interpreter of a target, or nil
// when the target is not modeled; such a target adds nothing, not even an
// issue. Build sets Evidence of every entity from its signal, replacing what
// the interpreter set, and passes the entities to New. A modeled target
// without a signal, with a failed read or whose interpreter fails adds an
// Issue instead of entities.
func Build(summaries []source.TargetSummary, interpreterOf func(source.Target) Interpreter) *Model {
	var entities []Entity
	var issues []Issue
	for _, s := range summaries {
		interpret := interpreterOf(s.Target)
		if interpret == nil {
			continue
		}
		issue := func(text string) { issues = append(issues, Issue{Target: s.Target.Name, Error: text}) }
		switch {
		case s.Latest == nil:
			issue("no signal collected yet")
			continue
		case s.Latest.Error != "":
			issue("read failed: " + s.Latest.Error)
			continue
		}
		read, err := interpret(*s.Latest)
		if err != nil {
			issue("interpret: " + err.Error())
			continue
		}
		evidence := Evidence{Target: s.Target.Name, URL: s.Latest.URL, ObservedAt: s.Latest.ObservedAt}
		for _, e := range read {
			e.Evidence = evidence
			entities = append(entities, e)
		}
	}
	return New(entities, issues)
}
```

### Issue texts

| Situation | `Issue.Target` | `Issue.Error` |
| --- | --- | --- |
| modeled target without a signal | target name | `no signal collected yet` |
| failed read | target name | `read failed: <Signal.Error>` |
| interpreter error | target name | `interpret: <err>` |
| empty ID | `Evidence.Target` | `entity of kind "<kind>" named "<name>": empty id` |
| empty Kind | `Evidence.Target` | `entity "<id>": empty kind` |
| Health not in the set | `Evidence.Target` | `entity "<id>": unknown health "<health>"` |
| link with empty Type or To | `Evidence.Target` | `entity "<id>": link <index>: empty type or target` |
| duplicate ID | `Evidence.Target` of the dropped entity | `entity "<id>": already read from target "<target of the kept entity>"` |

### Package documentation

`model/doc.go`:

```go
// Package model is the Navigate part of the Inspector toolkit (Navigate ->
// Model): it organizes the signals of a Source into entities, the links
// between them, their properties and their state.
//
// # Entities
//
// An Entity is one thing of the inspected system: its ID, Kind, Name, State,
// Properties and Links, and the Evidence of the signal it was read from. The
// ID is unique in a Model and the same in every build, so a view can link to
// an entity across refreshes. A Link points from the entity that holds it to
// another ID; Backlinks show the same link from the other end. A link may
// point to an ID that no signal described: it is kept, and Entity reports
// that the ID is not in the model.
//
// State is what the system says about the condition of the entity: Value and
// Reason in its own words, and Health, the only closed set of the package
// (unknown, ok, degraded, down), so that views can color it.
//
// # Building a model
//
// Build reads the latest signal of every summary of a source.Source with the
// Interpreter that the caller selects per target. An interpreter knows one
// kind of signal, for example the JSON list of one API, and returns the
// entities it describes. Build records the Evidence of every entity and
// hands them to New, which validates them. New is also the way to build a
// model from entities that are already interpreted.
//
// A Model is a snapshot built from the latest signals: entities of different
// targets may be observed at different times, and Evidence says when. It is
// read-only and safe for concurrent reads.
//
// # Issues
//
// Nothing that fails to be modeled is dropped silently. A target without a
// signal, a failed read, an interpreter error, an invalid entity and a
// duplicate ID each become an Issue with the target it came from. The rest
// of the model is still built.
//
// # Extending
//
// Kinds, link types and property names are defined by the interpreters;
// this package validates structure only and never branches on them. A new
// signal needs a new Interpreter. A new field of a type of this package has
// a zero value that means "not stated", so existing interpreters and views
// keep working; build values with keyed composite literals. A new query is a
// new method of Model. Health is closed on purpose: adding a value changes
// every view.
package model
```

### Invariants (tested)

- `Entities()` holds only valid entities with unique IDs, a non-empty `Name` and a `Health` from the set.
- The order of `Entities()` is: summaries in order, then the entities of each interpreter in their order.
- `Backlinks(id)` lists, for every kept entity in order and every link of it in order, `{From: entity ID, Type: link type}` when `link.To == id`.

## Verification

Commands:

```powershell
go test ./model/... -v
go-arch-lint check
go list -f '{{join .Imports " "}}' ./model
deadcode ./cmd/...
task all
```

Tests in `model/build_test.go` use `t0 := time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)`. They also use a helper `summary(name string, sig *source.Signal) source.TargetSummary`, which sets `Target{Name: name, URL: "http://example.test/" + name}`, and a helper `byName(map[string]model.Interpreter) func(source.Target) model.Interpreter`, which selects by `Target.Name`.

| Test | Asserts |
| --- | --- |
| `TestBuildReadsEntitiesInTargetOrder` | target `a` returns entities `x1`, `x2`; target `b` returns `y`; `Entities()` IDs are `[x1 x2 y]` |
| `TestBuildSetsEvidenceFromSignal` | the interpreter sets `Evidence{Target: "fake"}`; the entity has `Evidence{Target: "a", URL: "http://example.test/a", ObservedAt: t0}` |
| `TestBuildPassesSignalToInterpreter` | the interpreter records its argument; it equals `*Latest` with `StatusCode` 503, the content type and the body |
| `TestBuildSkipsTargetsWithoutInterpreter` | the selector returns nil for `a`; `Entities()` and `Issues()` are empty |
| `TestBuildReportsTargetWithoutSignal` | `Latest` is nil; `Issues()` is `[{a "no signal collected yet"}]` |
| `TestBuildReportsFailedRead` | `Latest.Error` is `connection refused`; `Issues()` is `[{a "read failed: connection refused"}]`; the interpreter is not called (it calls `t.Fatal`) |
| `TestBuildReportsInterpreterError` | `a` returns entity `x` and error `boom`, `b` returns `y`; `Issues()` is `[{a "interpret: boom"}]`; `Entity("x")` is not found; `Entity("y")` is found |
| `TestNewKeepsEntitiesInOrder` | `New` of `c`, `a`, `b` returns them in that order |
| `TestNewDropsInvalidEntities` | table over the five invalid cases of "Issue texts"; each gives the exact issue with `Target` from `Evidence.Target`, and the entity is missing from `Entities()` |
| `TestNewDropsDuplicateIDs` | `x` from target `a`, then `x` from target `b`; one entity, read from `a`; issue `{b, entity "x": already read from target "a"}` |
| `TestNewKeepsGivenIssuesFirst` | the given issue comes before the issue of an invalid entity |
| `TestNewReadsEmptyHealthAsUnknown` | an entity with empty `Health` has `HealthUnknown` |
| `TestNewNamesEntityByID` | an entity with empty `Name` has `Name == ID` |
| `TestModelFindsEntityByID` | `Entity("a")` returns the entity and true; `Entity("nope")` returns the zero Entity and false |
| `TestModelListsBacklinks` | `a` links `x` to `b`, `c` links `y` to `b`; `Backlinks("b")` is `[{a x} {c y}]`; `Backlinks("a")` is empty |
| `TestModelKeepsLinksToMissingEntities` | `a` links to `ghost`; the link stays in `a.Links`; `Entity("ghost")` is false; `Backlinks("ghost")` is `[{a <type>}]` |

## Acceptance Criteria

- Every test in the table exists in `model/` and passes.
- `go list -f '{{join .Imports " "}}' ./model` prints only standard library packages and `github.com/miroslav-matejovsky/inspector/source`.
- `.go-arch-lint.yml` contains the component `model` with `mayDependOn: [source]`, and `go-arch-lint check` passes.
- `taskfile/deadcode.ps1` lists the staged `model` names under the comment `# staged by dev/plans/model, removed in step 07`.
- The root `README.md` Library table has the `model` row.
- `task all` passes.

## Non-Goals

- No caller outside tests. Step 03 adds the interpreters and step 07 the pages.
- No signal history: `Build` reads `TargetSummary.Latest` only.
- No typed property values (numbers, times): `Property.Value` is text.
- No merging of one entity described by several signals: the first wins, the others are issues.
- No caching, persistence or incremental update of a model.
