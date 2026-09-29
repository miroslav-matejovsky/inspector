# Explain

The primary purpose of an Inspector is to explain system behaviour.

An Inspector exists to help users understand why the system is in its current state. It should make visible the reasons behind objects, relationships, decisions, outcomes and derived information without requiring users to study source code, logs or implementation details.

Explanation is not debugging. The goal is not to expose internal mechanics, but to provide meaningful answers in terms of the system's own concepts and model.

An Inspector should help answer questions such as:

- Why does this exist?
- Why is it in this state?
- Why is it related to that?
- Why was this conclusion reached?
- Why did this outcome occur?

A good Inspector reduces the distance between observable behaviour and human understanding. Users should be able to start from what they see and progressively discover the reasoning, dependencies and context that led to it.

If a user can observe a system but cannot understand why it behaves as it does, inspection has failed.