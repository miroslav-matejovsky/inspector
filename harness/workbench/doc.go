// Package workbench is the development harness for Inspector.
//
// It serves a single web page with one workbench panel, an empty placeholder
// for now. The page is embedded in the binary and served at "/".
//
// # Inspected simulation
//
// The workbench runs the simulated service of package inspected in the same
// process and mounts its handler under inspected.PathPrefix + "/" on the same
// HTTP server. There is no second entry point and no second port.
//
// # Configuration
//
// Config has no defaults. ParseConfig reads it from command-line flags and
// fails when any flag is missing:
//
//	-addr                       listen address, for example localhost:8080
//	-inspected-seed             seed of the traffic generator
//	-inspected-orders-per-tick  simulated orders placed per tick, 0..100
//	-inspected-tick-interval    wall-clock time between simulation ticks
//	-inspected-request-timeout  max wait for the simulation per HTTP request
//
// # Supervision
//
// Run is the supervisor of two goroutines: the inspected app and the HTTP
// server. When either one stops unexpectedly, Run cancels the other, waits
// for both and returns the error. Cancelling the context of Run is a clean
// shutdown and returns nil. Restart is not attempted. Inspected state is in
// memory, so a new start begins at tick 0 and replays the same traffic for
// the same seed.
package workbench
