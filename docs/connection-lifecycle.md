# Connections, Matchmaking, and Recovery

Games live in server memory independently of their WebSocket connections.
A profile ID is a placeholder identity, not authentication.

## Startup and resume

`GET /ws` sends `connection.ready` before processing commands. It contains a
new `profileId`, or the ID supplied in `?profileId=PROFILE_ID`, and an optional
`activeGameId` for that profile's unfinished game. The initial write has a
10-second deadline and is canceled on server shutdown.

Restart the TUI with `make run-tui PROFILE_ID=PROFILE_ID` to discover and resume
that game automatically. The running TUI does not reconnect automatically.
A raw WebSocket client uses the supplied game ID explicitly:

```json
{"type":"game.resume","requestId":"resume-1","payload":{"gameId":"GAME_ID"}}
```

An active resume returns `game.resumed`, then `game.initial`; the opponent gets
an unsolicited `game.state` showing the player online. A former participant can
also resume a known finished game as a read-only final snapshot. Finished games
are not supplied as `activeGameId` and do not occupy a live session.

One profile may have only one unfinished game or waiting matchmaking entry.
Reusing an ID does not close the older connection or transfer its seat. Close
the old client before resuming from another connection.

## Disconnects

Disconnecting does not resign: clocks and play continue. The remaining player
receives `game.state` with the departing player's `connected` value set to false.
Native WebSocket pings run every 30 seconds with a 10-second timeout; a silent
network loss is detected by failed I/O or heartbeat rather than immediately.
There is no disconnect grace period or automatic rematch.

## Matchmaking races

A waiting entry is canceled when its connection ends. Reopening the client does
not requeue it. Pairing commits a game whose clock starts immediately, and each
client receives `matchmaking.found` followed by `game.initial`. The first initial
state may show the other player offline until its registration is published.

This example admits the first client before the second; either order is valid.
Each client always gets its entry acknowledgement before assignment and initial
state. Waiters acquire the dispatch mutex before registration and publication.

```mermaid
sequenceDiagram
    actor A as First client
    participant H as Handler
    participant S as GameService
    actor B as Second client

    A->>H: matchmaking.enter
    H->>S: EnterMatchmaking
    S-->>H: Pending ticket
    H-->>A: matchmaking.entered
    Note over H: Waiter retains reservation and waits outside dispatch mutex
    alt First client disconnects before pairing
        A->>H: Connection ends
        Note over H,S: Cancellation ends the waiter and removes the queue entry
        H->>H: Release session reservation
    else Second client enters the same pool
        B->>H: matchmaking.enter
        H->>S: EnterMatchmaking
        S->>S: Create game, start clock, fill both buffered tickets
        S-->>H: Ready ticket
        H-->>B: matchmaking.entered
        Note over H: Each waiter reads current state and registers its player
        H-->>A: matchmaking.found (gameId, color)
        H-->>A: game.initial (opponent initially offline)
        H-->>B: matchmaking.found (gameId, opposite color)
        H-->>B: game.initial (both online)
        H-->>A: game.state (opponent online)
    end
```

If disconnect happens after pairing commits, the game remains. Restarting with
the same profile ID discovers it through `activeGameId`, including when the
assignment response was lost. Cancellation before pairing removes the waiting
entry. One player's disconnect does not cancel the surviving player's game.

## Delivery and restart limits

Queue acceptance does not confirm receipt, so clients must recover an unknown
outcome rather than replay the action. Committed peer updates are attempted even
if the source response fails. Terminal publication releases sessions for another
game. See [delivery decisions](adr/0003-message-delivery-semantics.md).

A server restart loses live games, matchmaking entries, and sessions. Persistence
adapters exist but are not wired into gameplay. Component ownership and shutdown
are described in [ADR 0002](adr/0002-lifecycle-and-cancellation.md).
