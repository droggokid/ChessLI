# ADR 0001: Core and Adapter Boundaries

Status: Accepted.

## Context and decision

Gameplay must be usable and testable independently of network and storage
implementations. Dependencies point toward application behavior and domain
models; the executable composes the implementations.

Gameplay and domain code do not depend on concrete transports or database
implementations. Adapters translate between domain values and external
representations. Wire payloads, JSON details, and protocol errors are separate
from gameplay models and rules, even where their fields look similar. Stable
identity types may be shared across that boundary.

Application-service and persistence dependencies are consumed through focused
interfaces so callers can be unit-tested with mocks, independently of concrete
implementations and external infrastructure. Keep interfaces and their mocks
aligned when contracts change.

The current application contract is `gameplay.Service`; it can be implemented
without a transport dependency. Its name, interface ownership, and method
placement are implementation choices, not requirements of this decision.
Support packages do not bypass these boundaries by carrying transport state
into the core.

## Rationale and trade-offs

Serializing gameplay models directly or calling transport implementations from
the core would reduce mapping code, but couple internal refactoring to clients
and make the core harder to reuse. Explicit boundaries cost conversion and
composition code, including some intentional representation duplication.

This decision protects dependency direction and separation of responsibilities,
not an exact package tree or a prescribed interface for each implementation.
The chess library is currently an intentional gameplay dependency; using its
types internally does not make them part of the wire contract.
