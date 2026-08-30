# ADR 0007: Prefer the Standard Library and Keep Dependencies Internal

Status: Accepted

## Scope

Applies to Go module dependencies, package APIs, tools, and generated code.

## Context

Adding a Go dependency is easy, but each dependency expands the project's
upgrade, security, compatibility, and maintenance surface. Types exposed across
package boundaries can also make a dependency difficult to replace.

## Decision

Use the standard library when it provides a reasonable solution. Add or retain
an external dependency only when it provides a concrete correctness,
maintainability, or functionality benefit that outweighs its ownership cost.

Third-party types remain implementation details unless adopting them as part of
a package contract is deliberate.

### Constraints

- Do not add dependencies for trivial functionality that is clear and
  maintainable with the standard library.
- Reuse an appropriate existing dependency before adding an overlapping one.
- A new dependency must have a concrete justification in the change that adds
  it.
- Exported APIs must not expose third-party types unless that dependency is an
  intentional part of the package contract.
- Transport packages map third-party or gameplay-specific values to protocol
  types rather than exposing them on the wire by accident.
- Tool and test dependencies must be pinned or otherwise reproducible through
  the module and repository tooling.
- Run `go mod tidy` only when dependency or import changes require it, and
  review both `go.mod` and `go.sum` changes.

### Invariants

- An internal implementation dependency can normally be replaced without
  changing the WebSocket protocol.
- Dependency additions are visible and reviewable in module metadata.
- Generated mocks can be reproduced from repository directives.

## Existing Decisions

The chess engine library is intentionally used by `internal/gameplay`, and
some gameplay package APIs currently expose its types within the `internal/`
tree. This does not make those types part of the WebSocket contract and is not
blanket approval to expose the dependency from other package APIs.

The WebSocket, UUID, and mock-generation dependencies each provide focused
functionality not reproduced locally.

## Alternatives Considered

### Implement every dependency locally

Rejected because:

- Reimplementing specialized protocols and chess rules would increase
  correctness and maintenance risk.

Rejected despite:

- It would minimize third-party module metadata.

### Add libraries whenever they reduce immediate code volume

Rejected because:

- Code volume alone does not account for upgrades, transitive code, security,
  or API coupling.

Rejected despite:

- A library can accelerate the first implementation.

## Consequences

Positive:

- The dependency surface remains deliberate and reviewable.
- Internal changes are less likely to force protocol or consumer changes.
- Standard-library knowledge remains broadly applicable to the codebase.

Negative:

- Some small utilities are implemented locally.
- Dependency proposals require explicit trade-off analysis.
