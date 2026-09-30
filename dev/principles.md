# Inspector Principles

## Observe

The Inspector exists to make systems observable through their externally visible signals.
Rather than relying on source code, internal implementation details, or direct access to runtime internals,
understanding should begin with what a running system chooses to expose about itself.
Metrics, APIs, events, logs, status endpoints, and other observable interfaces provide the
raw material from which insight is derived.
The Inspector encourages systems to communicate their state, behaviour, and structure through these signals,
making them discoverable without requiring implementation knowledge. The guiding question of observation is:

```text
What can the system tell us about itself?
```

Without observation there is nothing to inspect. The purpose of observation is to make information available for understanding.

### Navigate

The Inspector exists to make systems explorable. Understanding rarely emerges from isolated pieces of information.
Instead, it emerges by following the relationships that connect them.
An Inspector should help users move naturally between related entities, responsibilities, dependencies, states,
and sources of information, gradually building a coherent mental model of the system.
Every inspected element should provide context through its connections to other elements,
allowing users to understand not only individual facts but also how those facts fit together.
The guiding question of navigation is:

```text
How is this connected?
```

Without navigation, information remains fragmented. The purpose of navigation is to transform observations into context.

### Explain

The Inspector exists to make systems understandable. It should help users understand not only what a system is doing,
but also how it is structured, how its parts interact, and why it behaves the way it does.
Understanding should emerge from observable signals rather than source code, implementation details, or tribal knowledge.
The Inspector encourages systems to expose their architecture through observation, making responsibilities,
relationships, dependencies, behaviour, and design decisions discoverable from the outside. The guiding question of explanation is:

```text
How does this system actually work?
```

Without explanation, inspection becomes data collection.
The purpose of explanation is to transform observable signals into architectural understanding.

### Proximity

The Inspector should remain close to both the system and its developers.
Understanding should be available where design, implementation, testing, and troubleshooting occur,
rather than requiring dedicated environments, operational tooling, or production access.
By reducing the distance between building a system and understanding it, the Inspector enables rapid feedback,
safer experimentation, and continuous learning throughout the development lifecycle.
The guiding question of proximity is:

```text
How can understanding be made available where the work is happening?
```

Without proximity, understanding becomes delayed, costly, and disconnected from development.
The purpose of proximity is to make insight immediate and accessible.

### Alignment

The Inspector should be organized around the process by which understanding is created.
Understanding begins with observation, develops through exploration of relationships, and culminates in explanation.
This progression should be reflected directly in the architecture of the toolkit:

```text
Observe  → Source
Navigate → Model
Explain  → View
```

Sources acquire information from observable signals.
Models organize that information into entities, relationships, properties, and state.
Views present those models in a form that humans can understand.

By aligning the toolkit structure with the process of understanding, the Inspector remains simple, predictable,
and extensible across domains.
Every capability should contribute to one of three concerns: observing the system, modeling the system, or explaining the system.
