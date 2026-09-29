# Dev

Working records that drive the evolution of this repository. They are not user or API
documentation. Architecture overview lives in the root `README.md`; package documentation
lives in each Go package's `doc.go`.

Each folder has its own `README.md` that defines its file format and holds its index.

| Folder | Purpose |
| --- | --- |
| [`principles/`](principles/README.md) | Core design principles and guidelines for the repository. |
| [`assessments/`](assessments/README.md) | Evidence-backed reviews of the repository at a specific revision. |
| [`backlog/`](backlog/README.md) | Open implementation and tuning work not yet planned, one item per file. |
| [`bugs/`](bugs/README.md) | Confirmed defects awaiting a fix, one bug per file. |
| [`evaluations/`](evaluations/README.md) | Technical evaluations and decisions that support code evolution. |
| [`investigations/`](investigations/README.md) | Problem analyses usually on live system runtime, for example performance issues. |
| [`plans/`](plans/README.md) | Active implementation plans. Executable specifications split into numbered steps. |

## Flow

- Principles guide the overall design and decision-making process. Always refer to them when making changes or adding new features.
- Assessments and evaluations produce findings and decisions.
- Findings become bugs or backlog items.
- Backlog items and bugs selected for work become plans.
- Finished work is removed. Git history is the record of completed work.
- Actionable items outside these records, such as live verification, go in the root `.todo`.
