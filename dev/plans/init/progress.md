# Progress

Mark a step `done` when all its acceptance criteria are verified.

| Step | Title | Status |
| --- | --- | --- |
| [01](01-library-boundary.md) | Library boundary guard | done |
| [02](02-observation-model.md) | Observation model | done |
| [03](03-navigation-links.md) | Navigation links | done |
| [04](04-connectivity-http-reader.md) | Connectivity HTTP reader | done |
| [05](05-harness-adapter.md) | Harness adapter | done |
| [06](06-representation-json-views.md) | Representation JSON views | done |
| [07](07-workbench-integration.md) | Workbench integration and documentation | done |

Deviation in step 07: `workbench.Run` had a race. When `ctx` was already cancelled, `select` could pick the ready `appDone` case, and the app's nil return on cancellation was reported as `unexpected stop`. Building the inspector before the `select` made the race frequent in `TestRunStopsOnContextCancel`. Fix: an app stop with a nil error after `ctx` is cancelled counts as a clean shutdown.
