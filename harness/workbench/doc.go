// Package workbench is the development harness for Inspector.
//
// It serves web pages with one workbench panel that show the inspected
// system as a model built from the signals collected by the Source of the
// workbench, and the signals themselves in the raw view of package view/raw.
// The page template is embedded in the binary.
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
// # Inspected model
//
// The workbench models the inspected system with package model, from the
// signals of its Source only. A target is interpreted when the path of its
// URL is one of these observation endpoints of inspected; other targets,
// such as the metrics, are not modeled:
//
//	/inspected/                  service: name, description; link readiness
//	/inspected/health/ready      readiness and one check per check; links check, determined by
//	/inspected/api/dependencies  dependency per dependency, state = mode
//	/inspected/api/products      product per product with its stock numbers
//	/inspected/api/orders        order per order, newest first; links product, waits on, decided by
//
// Entity IDs are the paths that inspected states as self links, a check is
// "<readiness path>#<check name>". Health follows the words of inspected:
// up/healthy is ok, degraded/slow is degraded, down/outage is down; an order
// is down when it failed and ok otherwise; products state no health.
// Status 503 is accepted for readiness only; any other status, or a body
// that is not the expected JSON, is a model issue of the target.
//
// # Pages
//
// The panel header has a tab per page; the tab of the page shown is marked.
//
//	/                 dashboard.Render of the inspected model
//	/model            explorer.Index of the inspected model
//	/model?entity=ID  explorer.Entity of the entity ID; 404 when it is not in the model
//	/raw              raw.Render of the target summaries of the Source
//
// Every page builds the inspected model, or reads the summaries, when it is
// rendered, and includes the styles of every view. When the summaries cannot
// be read or a view cannot be rendered, the page shows the error instead,
// with status 500.
//
// Every 2 seconds a script of the page gets the current page again and
// replaces the panel in place, so the page does not reload. The scroll
// positions of the elements with a data-scroll key are kept. A failed
// refresh is shown in the panel header and the old panel stays. Without
// JavaScript, the page reloads itself every 2 seconds instead.
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
// redirects with 303 to the page given by the form value return, which every
// form carries. A return value that is not the path of a workbench page, for
// example another host, is rejected with 400 before inspected is called. A
// rejected change answers with the
// status and body of inspected as plain text. Both calls go to the inspected
// handler in process. The script of the page posts the forms with fetch and
// shows the page of the redirect in place, or the rejection in the header.
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
