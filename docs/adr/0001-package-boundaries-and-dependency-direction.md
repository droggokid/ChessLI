# ADR 0001: Package Boundaries and Dependency Direction

Status: Accepted

## Scope

Applies to all Go packages under `internal/` and to dependency wiring in
`cmd/`.

## Context

ChessLI contains gameplay rules and orchestration, identity types, WebSocket
protocol models, and transport-specific connection handling. Without an
explicit dependency direction, WebSocket or infrastructure details can leak
into gameplay code and make the core harder to test, reuse, and replace.

## Decision

Dependencies point toward the gameplay and identity core. The executable is
the composition root and may wire core and transport implementations together.

The intended direction is:

```text
cmd -> websocket -> gameplay -> identity
                 \-> identity
```

`internal/websocket/protocol` is part of the WebSocket boundary, not part of
the gameplay core. Support packages such as `config` and `log` remain outside
the domain layering and must not become back doors for transport state.

### Constraints

- `internal/gameplay` and `internal/identity` must not import WebSocket, HTTP,
  database, or framework-specific packages.
- Transport packages may depend on gameplay and identity packages; gameplay
  and identity packages must not depend on transport packages.
- Conversion between wire and gameplay representations happens in the
  WebSocket boundary.
- Avoid generic packages such as `utils`, `helpers`, or `common`.
- A new package must represent a meaningful responsibility or boundary.
- Cross-cutting wiring belongs in `cmd`, not in a core package.

### Invariants

- Gameplay behavior can execute without a network transport.
- Replacing the WebSocket implementation does not require changing gameplay
  rules.
- No import cycle is resolved by moving transport concerns into the core.

## Alternatives Considered

### Organize primarily by technical utility

Rejected because:

- Generic shared packages obscure ownership and tend to accumulate unrelated
  behavior.
- They make dependency direction difficult to infer and enforce.

Rejected despite:

- Shared helpers can reduce small amounts of local duplication.

### Let domain packages call transport implementations directly

Rejected because:

- It couples gameplay behavior to connection and protocol details.
- It prevents the core from being exercised independently.

Rejected despite:

- It can reduce explicit mapping and wiring in small implementations.

## Consequences

Positive:

- Package ownership and dependency direction remain visible.
- Core behavior stays independently testable.
- Transport implementations can evolve or be replaced at the boundary.

Negative:

- Boundary mapping and composition code are required.
- Some similar-looking types exist in different packages for different
  purposes.
