# ADR 0005: Interface Ownership Follows the Abstraction

Status: Accepted

## Scope

Applies to Go interfaces under `internal/` and to generated mocks derived from
them.

## Context

Go interfaces are satisfied implicitly. Small behavioral abstractions usually
exist because a particular consumer needs substitution. A deliberate
application service port is different: it defines the use cases the application
offers to multiple transports or clients.

Treating both kinds of interface identically either pushes an application
contract into one transport package or encourages every implementation to
publish an unnecessary mirror interface.

## Decision

Prefer concrete types. When one consumer needs a narrow capability, define the
smallest useful interface in, or as close as practical to, that consumer.

An intentional application service port may be owned by the application
package. `gameplay.Service` is ChessLI's application port: it defines the
gameplay use cases offered to WebSocket and future transports, while
`gameplay.GameService` is the concrete in-memory implementation.

### Constraints

- Do not create an interface solely because a struct exists.
- Consumer-specific interfaces are owned by the package that consumes the
  behavior.
- An application-owned service interface must represent an intentional layer
  boundary and a coherent set of use cases, not every method on an
  implementation.
- `gameplay.Service` contains application operations only; implementation
  lifecycle and configuration methods such as `Close` and
  `SetGameExpiredHandler` remain on the concrete type.
- Keep capability interfaces as small as the consumer permits.
- Do not widen an interface for a method that its existing consumer does not
  use.
- Avoid service interfaces that combine unrelated operations.
- Constructors return concrete types unless callers need an abstraction at
  construction time.
- Keep generated mocks near the consumer that uses the test seam, even when the
  application owns the interface.

### Invariants

- Implementations satisfy interfaces without explicit declarations or reverse
  transport dependencies.
- `gameplay.Service` can be implemented and tested without importing a
  transport package.
- A capability interface changes because its consumer's required behavior
  changes. An application service port changes because the application's
  offered use cases change.

## Alternatives Considered

### Define an interface beside every provider

Rejected because:

- Automatic provider-owned interfaces tend to mirror full implementations.
- They confuse an implementation inventory with an intentional application
  port.

Rejected despite:

- A deliberate application port can provide a useful shared contract.

### Put the complete gameplay port in the WebSocket package

Rejected because:

- It makes one transport appear to define the application's supported use
  cases.
- Future transports would depend conceptually on a WebSocket-owned contract or
  duplicate it.

Rejected despite:

- It follows consumer ownership literally for the current single transport.

### Introduce interfaces for all exported structs

Rejected because:

- It adds indirection without a demonstrated substitution need.
- It increases API and mocking maintenance.

Rejected despite:

- It may make future substitution look easier initially.

## Consequences

Positive:

- Interfaces stay small and tied to real use cases.
- The application service contract remains transport-independent.
- Tests substitute only the behavior they exercise.

Negative:

- Different consumers may define overlapping small interfaces.
- The application service port must be kept coherent rather than growing into
  an inventory of every concrete method.
- Interface changes require regenerating consumer-owned mocks and updating
  their tests.
