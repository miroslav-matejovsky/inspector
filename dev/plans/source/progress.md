# Progress

Mark a step `done` when all its acceptance criteria are verified.

| Step | Title | Status |
| --- | --- | --- |
| [01](01-signal-store.md) | Signal store and SQLite dependency | done |
| [02](02-log-file.md) | Log file | done |
| [03](03-source-package.md) | Source package | done |
| [04](04-workbench-logging.md) | Workbench logging | done |
| [05](05-workbench-source-and-text-view.md) | Workbench source and text view | done |

Changes from the plan, found while implementing:

- `source.Collect` drops a round when its context ends during the reads: nothing is stored and no target is logged as failing. Without this, every shutdown logged `source target failing ... context canceled` for each target. Regression check in `TestRunStopsWhenContextEnds`.
- `source.read` sets `Duration` through a named result; new test `TestCollectMeasuresDuration`.
- `workbench.Run` opens the Source with `context.WithoutCancel(ctx)`, so a context cancelled before startup still starts and then shuts down cleanly, as before.

Remaining: the live checks in `.todo` (sections `## Workbench` and `## Source in the workbench`).
