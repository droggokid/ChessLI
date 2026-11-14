# ADR-0002: Return matchmaking outcomes to the admission owner

## Status

Accepted

## Date

2026-11-14

## Context

One queued player's lifetime was spread across a result channel, context
cancellation channel, cleanup callback, callback stop function, cancellation
scan, transport waiter goroutine, and wait group. Following admission through
assignment required tracing both service and transport background work.

The handler already serializes admission and publication. It can process the
pairing result during the second entry without starting a waiter for either
player. Gameplay must remain independent of WebSocket sessions.

## Decision

`EnterMatchmaking` returns a `MatchmakingResult` immediately. It contains an opaque
`MatchmakingEntry` handle for the current admission. When a pair commits, it also
contains the opponent's entry handle and the game ID. Colors and current game
state are read through the existing gameplay operations.

The caller owns delivery to both entries. The WebSocket handler retains queued
entry handles with their contexts, sessions, and request IDs. On the second
entry, it sends the entry acknowledgement, then admits the waiting player and the
arriving player under the dispatch mutex. Failure to acknowledge or admit one
player does not prevent an attempt to admit the other. Each player's own context
controls delivery; disconnect after commitment leaves the game recoverable.

Callers explicitly remove abandoned entries with `CancelMatchmaking`. It removes
only the supplied entry if still queued. Repeated cancellation, nil handles,
late cancellation after pairing, and cancellation of a replaced entry are no-ops.
Pointer identity distinguishes admissions without adding a separate ID generator.

Queued entries retain their admission context. Before admitting another player,
the service removes canceled entries so they cannot be paired or block immediate
re-admission. Cancellation alone does not schedule cleanup: callers must use
`CancelMatchmaking` for prompt removal when there is no further activity. Service
shutdown also clears the queue.

The handler cancels queued entries during disconnect, including server shutdown.
Connection shutdown already waits for disconnect cleanup, so matchmaking no
longer needs a separate handler shutdown method or wait group.

## Alternatives considered

Keeping result channels and callbacks preserves automatic context cleanup but
retains split ownership and the extra background lifecycle.

A blocking matchmaking operation inside the handler would hold the dispatch
mutex while waiting, preventing the opponent from entering. Moving the blocking
call into a goroutine retains the waiter lifecycle.

Replacing callbacks with interfaces changes the call syntax without removing
the asynchronous coordination.

## Consequences

The wire messages and recovery rules stay the same: entry acknowledgement comes
before assignment and initial state, and a committed game survives lost replies.
The waiting player's initial state can show the arriving player offline until
the arriving player's registration is published.

The service API changes. A coordinator must own delivery for every admission in
its queue and must explicitly cancel abandoned entries. Independent callers
cannot expect notifications from the service. Existing server usage has one
handler coordinating all admissions.

Matchmaking no longer allocates result channels, registers cleanup callbacks, or
starts transport waiters. Context eligibility checks and exact-entry cancellation
preserve admission safety while making ownership explicit.
