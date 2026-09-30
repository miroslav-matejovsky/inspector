## Core Rules

- Never commit changes. NEVER!
- Be brave; Be Honest; Be kind; Be true;
- Be pragmatic and don't overcomplicate things, focus on delivering value not perfection.
- Communicate clearly and directly. Short sentences. Simple language.
- No praise. No filler. No fluff. No motivational text.
- Be concise in output. Be thorough in reasoning.
- Think before acting.
- Read existing files before changing code.
- Prefer editing existing files over rewriting whole files.
- Do not re-read files already read unless file may have changed.
- Keep solutions simple and direct.
- When unclear, explain problem and ask for clarification, or write note to local file and stop.
- Before task complete and code changes are made, run `task all` and `task all` must pass.
- Experimentation phase. Favor progress and clean code over backwards compatibility.
- Bigger decisions must be evaluated against principles defined in `dev/principles` folder.

## Documentation

- Use root `README.md` for project overview and architecture.
- Read documentation before changing code.
- Keep documentation close to code and synchronized with the implementation.
- Outdated documentation is considered a defect.
- Focus documentation on the package or folder's actual contents, responsibilities, and purpose.
- Avoid describing what it does not contain or what it is not; document the implemented structure and its intent.

### Package / Namespace Documentation

- Go package: use `doc.go`, no `README.md` in Go packages.
- Other folders contain `README.md` when no suitable documentation exists.
- If documentation is missing, create it.
- If design changes, update documentation in the same change.

### Code Documentation

- Document public APIs.
- Document non-obvious internal behavior.
- Document assumptions, constraints, and invariants.
- Document significant data structures and fields.
- Documentation must be self-contained.
- Do not require external documents to understand code.

## Architecture and Design

- Prefer boring, pragmatic solutions.
- Prefer readability over cleverness.
- Use TDD (Test-Driven Development) and think in terms of tests first.
- Follow KISS and YAGNI.
- Prefer composition over inheritance.
- Accept duplication until a pattern appears at least 3 times.
- Avoid premature abstraction and premature optimization.
- Fail fast.
- Fail visibly.
- Fail within well-defined supervision boundaries.
- Validate messages, inputs, and configuration at boundaries.
- Small changes preferred over large rewrites.
- Treat application configuration files as an insight into system behavior, not just as a source of values.
- No defaults in configuration files are allowed.
- All configuration must be explicit and documented in the configuration files.
- Backward compatibility and versioning are overrated. Favor progress and clean code over maintaining old behavior.

## Error Handling

- Never swallow errors.
- Return meaningful errors with context.
- Log only when action can be taken or information is valuable.
- Preserve original error details whenever possible.
- Distinguish business failures from system failures.
- Do not use logging as error handling.
- Avoid duplicate logging across layers.

## Testing

- Tests should be written before the corresponding implementation (TDD).
- New behavior requires tests.
- Bug fixes require regression tests.
- Keep tests deterministic.
- Avoid sleeps and timing dependencies.
- Prefer simple unit tests over heavy integration tests.
- Test observable behavior, not implementation details.
- NEVER start and verify on real running systems, not even on local or developer environments
- Verification on real running systems should be documented in the '.todo' file at the repository root.

## Go Specific

### Tooling

- Use `gopls` for navigation and references.
- Use `go doc` for library and package exploration.
- Nothing in `vendor` may be modified or committed.
- Treat third-party dependencies as read-only.

### Style

- Prefer idiomatic Go.
- Keep interfaces small.
- Define interfaces near consumers.
- Accept concrete types until abstraction is needed.
- Favor explicit code over generic frameworks.
- Keep package boundaries clear.
- Use contexts correctly.
- Do not store contexts in structs.
- Pass contexts explicitly.
- Channels are transport mechanisms, not architecture.
- Prefer explicit dependencies over service locators.
- Keep package APIs small and intentional.

### Documentation

- Document exported functions, types, constants, and variables.
- Document important internal functions when behavior is non-obvious.

### Testing

- Prefer table-driven tests when helpful.
- Use `require` from `testify` for assertions.
- Prefer deterministic tests over timing-based tests.
- Verify observable contracts, not internal implementation choices.

## Output

- Return code first.
- Explain only when needed.
- No boilerplate unless requested.
- No em dashes.
- Use plain ASCII punctuation.
- Code must be copy-paste safe.
- Return minimum output needed for task.
- If implementation is incomplete, document remaining work in `.todo` at repository root.
