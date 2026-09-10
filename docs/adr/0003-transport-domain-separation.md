# ADR 0003: Transport and Domain Models Are Separate

Status: Accepted

## Scope

Applies to `internal/websocket`, `internal/websocket/protocol`,
`internal/gameplay`, and `internal/identity`.

## Context

WebSocket messages represent an external wire contract. Gameplay models
represent application state and behavior. Although some structures and value
types look similar, the two sets of models evolve for different reasons.

## Decision

Wire representations and gameplay representations are separate concepts.
Protocol envelopes, payloads, message names, error codes, and JSON details
belong to `internal/websocket/protocol`. The WebSocket boundary decodes,
validates, maps, and encodes them.

Stable identity value types may be referenced from protocol models because the
dependency still points toward the core. Sharing an identity type does not make
a wire payload a gameplay model.

### Constraints

- New JSON tags and wire-only fields belong in
  `internal/websocket/protocol`, unless serialization is deliberately part of
  another package's contract.
- WebSocket packages perform JSON decoding and encoding.
- Mapping between protocol and gameplay representations occurs in
  `internal/websocket`.
- Gameplay logic must not depend on protocol message types, JSON field names,
  or a concrete WebSocket implementation.
- Similarity between a protocol payload and a gameplay command or result is not
  sufficient reason to merge the types.

### Invariants

- Gameplay logic remains usable without WebSockets.
- Changing a JSON representation does not inherently require changing gameplay
  behavior.
- Protocol validation failures are translated at the transport boundary rather
  than becoming gameplay rules.

## Alternatives Considered

### Serialize gameplay models directly

Rejected because:

- Internal refactoring could accidentally change the wire contract.
- Protocol compatibility concerns would leak into gameplay code.

Rejected despite:

- It reduces mapping code and superficially similar structs.

## Consequences

Positive:

- Wire compatibility and gameplay evolution can be reviewed independently.
- Protocol validation and mapping have an explicit owner.
- The gameplay package stays transport-independent.

Negative:

- Explicit conversion code is required.
- Some representation-level duplication is intentional.
