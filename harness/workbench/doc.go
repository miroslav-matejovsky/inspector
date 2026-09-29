// Package workbench is the development harness for Inspector.
//
// It serves a single web page with two panels:
//
//   - inspected: the system under inspection.
//   - inspector: the Inspector view of that system.
//
// Both panels are empty placeholders for now. The workbench only owns the
// HTTP server and the static page. The page is embedded in the binary.
//
// Run starts the server and blocks until its context is cancelled. The listen
// address is always supplied by the caller; there is no default.
package workbench
