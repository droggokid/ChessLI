# ADR 0004: Explicit Protocol Compatibility

Status: Accepted.

## Decision

Internal refactors preserve message names, payloads, errors, validation, and
observable ordering. Intentional protocol changes update the examples in
[README](../../README.md) and [manual error testing](../manual-error-testing.md).

- Unknown client message types return `unknown_message_type`.
- Unknown fields in recognized client payloads are rejected.
- Unknown top-level client envelope fields are ignored.
- Direct responses preserve `requestId`; unsolicited and peer messages omit it.
  Matchmaking retains the entry request ID through assignment.
- The TUI accepts additive server fields and reports malformed known messages.

Prefer optional, additive server fields. Client payload additions must account
for older servers rejecting unknown fields.

## Rationale

Strict payloads catch misspellings, while tolerant envelopes allow new metadata.
Client and server releases can differ, so compatibility must be deliberate.
[Delivery guarantees](0003-message-delivery-semantics.md) remain separate from
message compatibility.
