# Investigations

Problem analyses, usually of live system runtime behavior, for example performance
issues, resource growth, or unexpected message flow. An investigation answers a
specific question from captured evidence. It does not change code.

Each investigation has its own folder. The folder `README.md` is the authoritative
record. Captures, logs, profiles, and notes are stored next to it.

```text
investigations/
├── README.md              # Index of all investigations
├── <investigation-topic>/
│   ├── README.md          # Question, evidence, findings, and outcome
│   ├── captures/          # Raw logs, traces, profiles, metrics
│   └── ...
└── ...
```

Each investigation `README.md` is self-contained and has a yaml front matter section
with the following fields:

```yaml
status: "Open|Concluded"
date: "YYYY-MM-DD"
revision: "git revision of the investigated build"
```

Then the file continues with:

- **Question** - the problem being investigated, stated as one answerable question.
- **Context** - environment, configuration, and load under which it was observed.
- **Evidence** - the captures used, cited by path, with relevant excerpts.
- **Findings** - what the evidence shows, separated from assumptions.
- **Outcome** - the answer, and the bugs, backlog items, or `.todo` actions it produced.

Investigations work from captured data. Collecting new data on a running system is an
action for the root `.todo` file. Remove a concluded investigation once its outcomes
are tracked elsewhere.

## Investigations

| Investigation | Status | Date |
| --- | --- | --- |
