// Package source collects observable signals from a running system and
// stores them.
//
// It is the Observe part of the toolkit (Observe -> Source). It takes what a
// system exposes through HTTP GET endpoints, raw and unparsed; interpreting
// signals is left to the Model. A Source never sends anything but GET
// requests without a body.
//
// # Signals
//
// A Signal is one read of one Target in one round: target name and URL, start
// time, duration, status code, content type and body. A response with any
// status code, including 4xx and 5xx, is a successful read: the status is
// part of the signal. A read fails, and Signal.Error is set with a nil body,
// when no response arrives within Config.Timeout, the body cannot be read,
// or the body is larger than Config.MaxBodyBytes. Failed reads are stored
// too: a system that does not answer is an observation.
//
// # Rounds
//
// Collect reads all targets concurrently and stores their signals in one
// transaction, in target order. After each round it deletes signals observed
// more than Config.Retention before the start of the round. Rounds never
// overlap. A round cut short by the end of its context is dropped: reads it
// interrupted are neither stored nor logged as failures. Run collects one round at once and then one per Config.Interval;
// ticks that arrive during a round are dropped.
//
// # Storage
//
// Signals are stored in the SQLite file at Config.DatabasePath through
// package internal/signalstore.
//
// # Logging
//
// Run logs "source started". A target logs "source target failing" once when
// its reads start failing and "source target recovered" once when they
// succeed again, not once per round.
//
// # Lifecycle
//
// Open the Source, run Run in a goroutine owned by the caller, and call Close
// after Run has returned. Run returns nil when its context ends and an error
// when the store fails. Summary is safe to call concurrently with Run.
package source
