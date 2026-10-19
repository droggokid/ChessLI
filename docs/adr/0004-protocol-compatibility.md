# ADR 0004: Explicit Protocol Compatibility

Status: Accepted.

## Context and decision

WebSocket messages are a contract between independently running clients and
servers. Internal refactoring preserves that contract; changes to names,
payloads, validation, errors, or observable flow are intentional and documented.

The existing compatibility rules remain:

- Unknown client message types return `unknown_message_type`.
- Unknown fields inside recognized client payloads are invalid messages.
- Unknown top-level client envelope fields are ignored.
- Direct responses preserve the initiating `requestId`. Unsolicited events
  and peer broadcasts do not reuse another client's request ID.

Prefer optional, additive server fields when practical. An additive client
payload field is incompatible with an older server that rejects unknown fields;
changes must account for that asymmetry.

## Rationale and trade-offs

Strict payload decoding catches misspellings and unsupported input, at the cost
of harder request evolution. Tolerant envelopes allow additional metadata.
Neither behavior should change accidentally because a Go struct moves or a
decoder is replaced.

Protocol examples describe implemented, reachable behavior and remain ready to
paste into `websocat`; errors not reachable through WebSocket are identified
as such. Update affected examples and compatibility guidance when behavior
changes. This does not require unrelated documentation edits or a separate
review ceremony.

[Delivery guarantees](0003-message-delivery-semantics.md) are recorded separately.
