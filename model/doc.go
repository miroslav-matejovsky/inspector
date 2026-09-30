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
