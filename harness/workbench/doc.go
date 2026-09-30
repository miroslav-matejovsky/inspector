// Package workbench is the development harness for Inspector.
//
// It serves a single web page with one workbench panel that shows the signals
// collected by the Source of the workbench in the raw view of package view. The page is embedded in the binary
// and served at "/".
//
// # Inspected simulation
//
// The workbench runs the simulated service of package inspected in the same
// process and mounts its handler under inspected.PathPrefix + "/" on the same
// HTTP server. There is no second entry point and no second port.
//
// # Source
//
// The workbench runs a source.Source against the inspected simulation. Its
// targets are paths on the workbench server, given as -source-target
// name=path; the Source reads them through the workbench listener with HTTP
// GET, like any external client. Signals are stored in the SQLite file given
// by -source-database.
//
// # Page
//
// The page shows view.Raw of the target summaries of the Source, styled by
// view.Styles, and reloads every 2 seconds. When the summaries cannot be read
// or rendered, the page shows the error instead, with status 500.
//
// # Controls
//
// The page header has one form per inspected dependency with a button per
// mode (healthy, slow, outage). The workbench reads the dependencies and their
// current modes from GET inspected.PathPrefix+"/sim" on every page render and
// marks the button of the current mode active; when the read fails, the header
// shows the error and the page answers 500. A button posts the mode to
// "/controls/dependencies/{name}"; the workbench sends it to the inspected
// control API, PUT inspected.PathPrefix+"/sim/dependencies/{name}", and
// redirects back to the page. A rejected change answers with the status and
// body of inspected. Both calls go to the inspected handler in process.
//
// # Configuration
//
// Config has no defaults. ParseConfig reads it from command-line flags and
// fails when any flag is missing:
//
//	-addr                       listen address, for example localhost:8080
//	-log-dir                    directory of the log files, created when missing
//	-log-level                  lowest level written to the log file: debug, info, warn, error
//	-inspected-seed             seed of the traffic generator
//	-inspected-orders-per-tick  simulated orders placed per tick, 0..100
//	-inspected-tick-interval    wall-clock time between simulation ticks
//	-inspected-request-timeout  max wait for the simulation per HTTP request
//	-source-database            SQLite file of the collected signals, directory created when missing
//	-source-interval            time between collection rounds
//	-source-timeout             max wait for one read of one target
//	-source-retention           signals older than this are deleted
//	-source-max-body-bytes      a larger body is stored as a failed read
//	-source-target              name=path of a workbench path to read, repeated
//
// # Logging
//
// cmd/workbench owns the log file: it opens logs/workbench-<time>.log from
// LogDir and LogLevel and passes its logger to Run. Run logs "workbench
// started" with the listen address and "workbench stopped" with the error, if
// any. The Source logs to the same logger.
//
// # Supervision
//
// Run is the supervisor of three goroutines: the inspected app, the Source
// and the HTTP server. When one stops unexpectedly, Run cancels the others,
// waits for all of them, closes the Source and returns the error. Cancelling
// the context of Run is a clean shutdown and returns nil. Restart is not
// attempted. Inspected state is in memory, so a new start begins at tick 0
// and replays the same traffic for the same seed; stored signals survive
// until their retention ends.
package workbench
