# ADR 0002: Session and Goroutine Lifecycle

Status: Accepted

## Scope

Applies to sessions, connections, matchmaking, timers, and other long-running
concurrent components.

## Context

ChessLI uses goroutines and asynchronous callbacks for WebSocket read and write
loops, HTTP server lifecycle, matchmaking cleanup, match notification, and
chess-clock expiration. Ambiguous ownership makes shutdown behavior difficult
to reason about and can leave background work running after its useful
lifetime.

## Decision

Every goroutine has one clear lifecycle owner. The component that starts a
goroutine is responsible for giving it a deterministic termination condition
and, when required by its lifecycle contract, waiting for it to finish.

The WebSocket session owns its read and write loops. The server owns active
connection lifetimes. Gameplay owns matchmaking cancellation callbacks and
game expiration timers.

Long-running concurrent work terminates through context cancellation, an
explicitly owned shutdown signal, or completion of owned input. Asynchronous
callbacks must be finite or cancelable.

### Constraints

- Code that receives from a channel must not close that channel unless it also
  owns the sending lifecycle.
- Every started goroutine must have an identifiable owner and deterministic
  termination condition.
- Cancellation callbacks must be stopped when their associated work completes
  before cancellation.
- A component that promises graceful shutdown must wait for the goroutines
  covered by that promise.
- Session shutdown uses cancellation and connection closure; receivers must not
  close the session's outgoing channel.
- Do not add synchronization merely to impose an ordering on otherwise valid
  concurrent outcomes.
- Lifecycle methods must be safe according to their documented concurrency
  contract.
- Tests for lifecycle behavior must use observable synchronization rather than
  arbitrary sleeps.

### Invariants

- `Session.Run` does not return until both session-owned I/O loops terminate.
- Server shutdown cancels connection lifetimes and waits for active connection
  handlers, subject to the shutdown context.
- A stopped game timer cannot later apply that timer's expiration to the game.
- A successful match does not leave dormant queue-cleanup work waiting for a
  later cancellation.
- Session shutdown does not leave session-owned background goroutines running.

## Alternatives Considered

### Let goroutines terminate with process exit

Rejected because:

- It makes tests flaky and resource ownership implicit.
- It prevents graceful server shutdown.

Rejected despite:

- It requires less lifecycle code.

### Add synchronization for every scheduling race

Rejected because:

- Synchronization is useful only when it strengthens a documented guarantee.
- Extra mutexes and atomics can hide ownership rather than clarify it.

Rejected despite:

- A single observed execution order can appear easier to reason about.

## Consequences

Positive:

- Shutdown behavior can be reasoned about from ownership.
- Concurrency tests can assert stable lifecycle guarantees.
- Goroutine leaks are less likely.

Negative:

- Owners need explicit cancellation, joining, or timer cleanup logic.
- Some concurrent outcomes remain intentionally unordered.
