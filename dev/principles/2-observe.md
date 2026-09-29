# Observe

The primary purpose of an Inspector is to provide visibility into the current state of a system.

An Inspector should allow users to see what the system knows, what exists within it, and how its concepts relate to one another at a given point in time. It serves as a window into the system, making its state accessible without requiring direct access to internal storage, logs or implementation details.

Observation is intentionally read-only. An Inspector should not control, modify, simulate or influence the system it observes. Its responsibility is understanding, not operation.

An Inspector should help answer questions such as:

- What exists?
- What is the current state?
- What relationships are present?
- What information is available?
- What does the system currently know?

A good Inspector provides a faithful representation of the system as it is, allowing users to build understanding from observation before moving on to explanation and analysis.

If users cannot reliably see and explore the state of the system, meaningful inspection is impossible.