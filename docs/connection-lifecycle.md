# Connection and matchmaking lifecycle

This describes the current in-memory WebSocket behavior. A connection is a
`Session`; a profile ID identifies a player across connections; a game remains
in gameplay memory independently of either connection.

## Connecting and detecting a lost connection

- `GET /ws` accepts a WebSocket and sends `connection.ready` with a `profileId`.
  With `?profileId=PROFILE_ID`, the server reuses that ID; otherwise it creates
  a new one. The ID is a placeholder identity, not authentication.
- Reusing a profile ID does not close an older connection. The server rejects
  a second queue entry for that profile and a second session in the same game;
  it does not automatically transfer the old session's place to the new one.
- Each session reads client messages, writes queued server messages, and sends
  WebSocket pings every 30 seconds with a 10-second timeout. A read, write, or
  ping failure, a normal close, or server shutdown ends the session. A silent
  network loss is noticed when an I/O operation or heartbeat fails, not
  necessarily at the moment the link drops.
- When the session ends, the server cancels its operations and removes it from
  the live game-session registry. If it belonged to a game, the remaining
  connected player is sent an unsolicited `game.state` showing
  `connected: false` for that profile. Disconnecting does not resign or end
  the game.

## Disconnecting around matchmaking

1. `matchmaking.enter` reserves the session and queues the profile in a pool
   for the exact time control. The server queues `matchmaking.entered` in reply.
   There is no separate cancel message; closing the connection cancels its
   waiting entry once the server detects the close. A later player should not
   match against an entry already removed for cancellation.
2. When another player enters the same pool, gameplay removes the waiting
   entry and creates a game for both profiles. Its clock starts immediately.
   Each live session then independently processes its match result, registers
   with the game, and queues `matchmaking.found` followed by `game.initial`.
   One player's initial state may briefly show the other as disconnected;
   registration of the second player sends a later `game.state` to the first.
3. If a connection drops just as pairing happens, the outcome depends on
   whether cancellation is observed before the match is created. Before match
   creation, the waiting entry is removed. After match creation, the game is
   **not** canceled or requeued: the surviving player can receive the match
   and see the other player disconnected. A result or outgoing message queued
   for the lost connection is not proof that it received the game ID.

There is no automatic rematch, disconnect grace period, or automatic resume.
A player who lost the connection after a match was created must know both the
same `profileId` and the `gameId` to resume it. If the disconnect happened
before the client received the game ID and it has no other way to obtain it,
the current WebSocket API cannot discover that match by profile ID.

## Disconnecting during a game and resuming

The game, its moves, and its clock continue in memory while a player is away.
The opponent can keep playing; the disconnected player's turn can time out.
To return, open `/ws?profileId=PROFILE_ID` and send:

```json
{"type":"game.resume","requestId":"resume-1","payload":{"gameId":"GAME_ID"}}
```

For an active game, the returning player receives `game.resumed` and the
current `game.initial` snapshot. The opponent receives an unsolicited
`game.state` with `connected: true`. A finished game can be resumed as a
read-only final snapshot without registering the session as connected to it.
When a game finishes, live sessions are released for new games. Opening a new
connection with the same profile ID alone does not resume a game.

## Delivery and restart limits

Server messages use a bounded outgoing queue. A successful enqueue does not
confirm a socket write or client receipt, and a message racing with disconnect
may be lost. Live games, matchmaking entries, and sessions are in memory; a
server restart does not recover an unfinished game or its queue.

Implementation: `internal/websocket/server.go`, `session.go`,
`handler_admission.go`, and `game_sessions.go`; `internal/gameplay/service.go`.
