# ADR 0006: Context Represents Operation Lifetime

Status: Accepted

## Scope

Applies to operations, goroutines, network I/O, waits, and shutdown paths under
`internal/` and `cmd/`.

## Context

Network and concurrent operations need cancellation and deadlines. Treating
contexts as configuration or retaining request contexts in long-lived objects
obscures ownership and can keep request-scoped values alive unexpectedly.

## Decision

`context.Context` represents the lifetime of an operation. Callers create,
cancel, and propagate operation contexts. Long-lived components may store a
component-owned lifecycle context only when they create it themselves and keep
its cancel function as part of explicit lifecycle management.

### Constraints

- Context is the first parameter when present.
- Do not store caller-provided or request-scoped contexts in long-lived structs.
- A component-owned stored context must have an owned cancel function and a
  documented lifecycle purpose.
- Blocking operations accept a context when cancellation is meaningful.
- Code checks or selects on cancellation at points where it can release owned
  resources or stop useful work.
- Cancellation directly caused by the caller returns `ctx.Err()` when no more
  specific error takes precedence.
- Context is not used for configuration, optional parameters, or general
  dependency injection.
- Do not replace a propagated operation context with `context.Background()`;
  background contexts are reserved for explicit process or component lifecycle
  roots and bounded shutdown work.

### Invariants

- Canceling an operation eventually releases resources and goroutines owned by
  that operation.
- Request cancellation does not accidentally cancel unrelated component work.
- Long-lived components do not retain request-scoped context values after the
  request lifetime.

## Alternatives Considered

### Store each caller's context on the receiving struct

Rejected because:

- It conflates one operation's lifetime with the component's lifetime.
- It can retain request values and make later calls depend on stale
  cancellation.

Rejected despite:

- It reduces explicit context parameters on internal methods.

### Use custom cancellation channels everywhere

Rejected because:

- Context already composes deadlines and cancellation across call boundaries.
- Parallel cancellation mechanisms make ownership harder to follow.

Rejected despite:

- An owned channel can be appropriate for lifecycle state that is not an
  operation context.

## Consequences

Positive:

- Cancellation flows follow call and ownership boundaries.
- Blocking operations and shutdown paths have consistent behavior.
- Request-scoped data is less likely to outlive its request.

Negative:

- Context must be threaded through cancellable call paths.
- Components with independent lifetimes require explicit root contexts and
  cancel functions.
