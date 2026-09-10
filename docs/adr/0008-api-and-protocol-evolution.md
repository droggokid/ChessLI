# ADR 0008: Protocol Changes Are Explicit and Compatibility Is Intentional

Status: Accepted

## Scope

Applies to WebSocket envelopes, message types, payloads, validation, errors,
observable delivery flow, and the documentation that describes them.

## Context

Client and server message formats are a contract between independently running
components. A small Go refactor can otherwise rename a JSON field, change
validation, or alter message ordering without being recognized as a protocol
change.

The current decoder rejects unknown fields inside recognized client payloads.
Unknown message types receive an application error. Top-level envelope decoding
uses Go's default JSON behavior and therefore ignores unknown envelope fields.

## Decision

Protocol changes are reviewed separately from internal refactoring. Backward
compatibility is a deliberate choice, not an accidental result of Go struct
layout.

The current compatibility behavior remains part of the contract until an
explicit protocol change says otherwise:

- Unknown client message types are rejected with `unknown_message_type`.
- Unknown fields inside a recognized client payload are rejected as an invalid
  message.
- Unknown top-level envelope fields are ignored during decoding.

### Constraints

- Renaming or reorganizing an internal Go field must not accidentally change a
  JSON field name or message type.
- New server response fields should be optional and additive when practical.
- Adding a field to a client request is not backward-compatible with older
  servers while unknown payload fields are rejected; plan such changes
  accordingly.
- Breaking changes to message names, required payloads, validation, errors, or
  observable message flow must be intentional and documented.
- Internal refactoring preserves the wire contract unless the task explicitly
  changes that contract.
- When protocol behavior changes, update both `README.md` and
  `docs/manual-error-testing.md` in the same change.
- Keep manual JSON examples valid for direct use with `websocat`.
- Application errors that cannot currently be reached through the WebSocket API
  must be identified as such in protocol documentation.

### Invariants

- Internal package refactoring does not implicitly redefine the client/server
  protocol.
- Clients can correlate direct responses through `requestId`; unsolicited or
  peer broadcast state does not reuse another client's request ID.
- Documented examples and compatibility rules describe the implemented
  protocol.

## Alternatives Considered

### Let Go struct changes define protocol evolution

Rejected because:

- Refactors would become accidental external contract changes.
- Compatibility impact would be difficult to review.

Rejected despite:

- It requires no separate protocol planning.

### Always accept unknown payload fields

Rejected because:

- The current protocol deliberately catches misspelled and unsupported input
  fields.

Rejected despite:

- Tolerant readers can make additive request evolution easier.

### Reject unknown fields everywhere

Rejected because:

- The current envelope decoder is intentionally documented as tolerant at the
  top level.

Rejected despite:

- Uniform strictness can appear simpler to describe.

## Consequences

Positive:

- Compatibility impact is visible during review.
- Internal refactors can proceed without silently changing clients.
- Manual protocol documentation acts as a practical verification aid.

Negative:

- Protocol changes require coordinated code, test, and documentation updates.
- Strict client payload decoding limits some additive request evolution.
