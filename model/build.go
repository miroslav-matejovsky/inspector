package model

import "github.com/miroslav-matejovsky/inspector/source"

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
