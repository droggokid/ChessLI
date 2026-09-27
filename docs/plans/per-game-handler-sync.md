# Per-game handler synchronization

Status: Proposal; no implementation changes.

## Why

Gameplay locks protect state, but release before the handler queues messages.
Without coordination across both steps, an older snapshot can be queued after
a newer one. The current global `Handler.dispatchMu` closes that gap but also
makes unrelated games wait for each other.

## Smallest next step

Replace global dispatch serialization with one transport-owned mutex per game.
Hold it across state access/change, session membership updates, and outgoing
message queue acceptance. Actual network writes remain outside the lock.

- Commands, join/resume, matchmaking admission, disconnect publication, and
  timeout publication must all use the same game's lock.
- Protect the lock registry with a short-lived mutex; release it before waiting
  for a game lock. Never hold two game locks at once.
- Keep existing session reservations: game locks alone cannot prevent one
  session from entering two different games. Creation reserves the session
  first; matchmaking waits for an assignment before acquiring a game lock.
- For disconnects, look up the game, acquire its lock, then recheck membership
  before removing it. Do not hold the session-map mutex while acquiring a game lock.
- Timer mutations still use gameplay locks. Asynchronous timeout publication
  must acquire the dispatch lock and refresh state, or explicitly reject stale
  snapshots; acquiring a lock does not make a previously captured snapshot fresh.
- Retain each lock while its game remains accessible. Removing a registry entry
  while callers still reference it can create two independent locks for one game.

This coordinates this server's transport operations, not multiple processes or
arbitrary direct service callers. Existing gameplay locks remain necessary.

## Persistence boundary

[ADR 0009](../adr/0009-persistence-boundary.md) persists completed games, not each
live move. A save failure must not undo completion or prevent final-state delivery.
My earlier suggestion to require a successful save before publishing does not
fit that decision.

Capture an immutable completed-game record and queue the final state under the
game's dispatch lock; perform database I/O outside it through application wiring,
not SQL in the handler. Log initial save failures as specified by the ADR. Define
one owner for completion saving so repeated reads/publications do not trigger
duplicate writes. Background saving, if chosen, needs bounded work and explicit
shutdown ownership—not an untracked goroutine per notification.

Durable move acknowledgements, retries, and restart recovery are separate
decisions. Per-game locks do not provide those guarantees.

## Before replacing the global lock

Update [ADR 0004](../adr/0004-message-delivery-semantics.md) with the new lock scope.
Add controlled concurrency tests proving that unrelated games proceed independently,
same-game updates stay ordered, terminal cleanup cannot race with admission, one
session cannot enter two games, and storage failure does not suppress final delivery.
Run the race suite. No worker-per-game or event framework is needed for this step.
