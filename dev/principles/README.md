# Principles

Principles capture the architectural and domain truths that define a system.
Unlike technologies, frameworks, implementation details or coding conventions, principles describe how a system understands its problem space and how important decisions should be made.
A good principle remains valid even if the implementation changes completely.

| Principle | Description |
|-----------|-------------|
| [Explain](1-explain.md) | Help users understand why the system is in its current state. Inspector makes decisions, relationships, outcomes and derived information understandable by exposing their reasons, dependencies and causes. |
| [Observe](2-observe.md) | Provide a faithful, read-only view of the current system state. Inspector reveals what exists, what is known and how information is related without modifying or influencing the system being inspected. |
| [Navigate](3-navigate.md) | Enable exploration of the system through its relationships. Inspector allows users to move between connected concepts, discover context, trace dependencies and understand how information is linked. |
| [Domain Map](4-domain-map.md) | The Inspector Domain is organized into five supporting domains: **Observation** (state visibility), **Explanation** (behaviour understanding), **Navigation** (relationship traversal), **Representation** (presentation of understanding) and **Connectivity** (integration with inspected systems). Together they support the principles of Observe, Explain and Navigate. |