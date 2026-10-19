# ADR 0002: Lifecycle Ownership and Cancellation

Status: Accepted.

## Context and decision

Connections, matchmaking, clocks, and notifications outlive individual calls.
Each goroutine, timer, and asynchronous callback has a clear lifecycle owner,
a termination condition, and cleanup appropriate to that lifetime. Graceful
shutdown waits for the work covered by its contract.

Operation contexts propagate cancellation and deadlines, not configuration or
dependencies. Canceling an operation eventually releases its owned resources
without canceling unrelated component work. Caller/request contexts and their
values are not retained by long-lived components after the operation ends.
Independent component work has an explicitly owned lifetime instead of silently
discarding a caller's cancellation. Cancellation controls queued matchmaking,
network and database I/O, and shutdown. Synchronous in-memory game operations
finish once dispatched; they do not poll caller cancellation before each state
change. Once committed, an action is not rolled back by a disconnect.

The lifecycle contracts are:

- Session execution returns only after its owned I/O and heartbeat loops stop.
  Shutdown safely stops concurrent producers and consumers without leaving
  session-owned work running or allowing sends to a closed channel.
- Server shutdown cancels connection lifetimes and waits for active handlers,
  subject to the shutdown context's deadline.
- Stopping or replacing a game timer prevents its stale expiration from
  changing the game. Successful matchmaking releases cancellation cleanup
  associated with waiting.
- Gameplay expiration notifications are asynchronous: service operations do
  not wait for handlers that may call back into transport. Handlers are finite
  or cancelable. Service shutdown stops new notifications and timers and joins
  started notification handlers. In the current API, `GameService.Close` is
  called after application operations stop, never from a notification handler.
- After a committed action, the acting request's cancellation does not suppress
  finite, non-blocking peer notification attempts
  ([ADR 0003](0003-message-delivery-semantics.md)). That independent lifetime
  does not extend to network I/O, which remains session-owned.

In the current implementation, `game` owns locking and timers around synchronous
`gameState` rules. `GameService` owns the matchmaking queue's cancellation work.
`Handler` owns command ordering and matchmaking waiters; its private admission
component owns no separate lock or goroutine.

## Rationale and trade-offs

Relying on process exit hides leaks and makes shutdown unreliable. Explicit
ownership costs cancellation and joining logic, but makes resource release
testable. Contexts and owned shutdown signals can both serve these contracts;
no particular channel, mutex, or cancellation helper is required, and unrelated
concurrent outcomes need not have a fixed order.
