// Package signalstore keeps the signals collected by package source in an
// embedded SQLite database.
//
// The driver is github.com/ncruces/go-sqlite3: pure Go, no cgo. Only this
// package imports it, so the database can be replaced here without touching
// package source.
//
// # File
//
// Open takes a file path, creates its directory when it is missing, and opens
// the file through a "file:" URI. Paths containing '?' or '#' are rejected
// because they would break the URI. The database runs in WAL mode, so other
// SQLite clients can read the file while it is written; while it is open, the
// file has "-wal" and "-shm" companions. A locked write waits up to 5 seconds
// (busy timeout) before it fails.
//
// # Schema
//
// One STRICT table, signals, with one row per record, indexed by target and
// by observation time. PRAGMA user_version holds the schema version, 1. A new
// file gets the schema; any other version fails Open. There are no migrations:
// delete the file to start empty.
//
// # Values
//
// Times are stored as Unix nanoseconds and read back in UTC. Durations are
// stored as nanoseconds. A nil body is stored as NULL; a nil or empty body
// reads back as nil.
//
// # Concurrency
//
// A Store uses one connection. Operations are safe for concurrent use and run
// one after another.
package signalstore
