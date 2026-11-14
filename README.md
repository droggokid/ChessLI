# ChessLI
ChessLI is a terminal-first chess platform inspired by Chess.com, built for playing games, managing matches, and exploring chess from the command line.

## Finding the code

`cmd/main.go` wires the gameplay service and WebSocket server. Each concrete
type keeps its methods in one file; helper files contain independent functions.

| Responsibility | Start here |
| --- | --- |
| Application operations, gameplay locking, timers, and shutdown | `internal/gameplay/service.go` |
| Matchmaking queue mechanics | `internal/gameplay/match_queue.go` |
| Game rules and synchronous state transitions | `internal/gameplay/game.go` |
| Clock arithmetic | `internal/gameplay/clock.go` |
| Connection acceptance and lifetime | `internal/websocket/server.go` |
| Socket I/O and heartbeat | `internal/websocket/session.go` |
| Command ordering, matchmaking admission, and creating/joining/resuming games | `internal/websocket/handler.go` |
| Membership, presence, and state publication | `internal/websocket/game_sessions.go` |

Wire decoding and mapping stay in `internal/websocket`; payload types live in
`protocol/`. Persistence lives in `internal/persistence`, with `GameRepository`
and `ProfileRepository` interfaces, implementations ending in `Impl`, generated
mocks, and database DTOs. The executable injects these repositories into gameplay;
gameplay explicitly maps its domain records to persistence DTOs. The repositories
use pgx pools and Squirrel query builders. A completed game includes its
ordered moves in a JSONB array and is saved in one insert. Live gameplay remains
in memory.

Handler orders commands and outgoing queue acceptance; GameService protects game
state and timer updates. Session writers perform the socket I/O.

```mermaid
flowchart LR
    C[Terminal client] <-->|WebSocket| S[Session]
    S -->|Commands| H[Handler: dispatch mutex]
    H -->|Queue responses| S
    H -->|Named operations| GS[GameService: gameplay mutex]
    GS --> G[Synchronous game rules]
    G --> CL[Clock arithmetic]
    GS -.->|Async timeout notification| H
```

See [connection recovery](docs/connection-lifecycle.md) and the
[future roadmap](docs/roadmap.md).

## System sequence

A successful private game from connection to session reuse. Responses below are
queued for delivery; messages to different clients can interleave.

```mermaid
sequenceDiagram
    actor W as White client
    participant S as ChessLI server
    actor B as Black client

    W->>S: Connect
    S-->>W: connection.ready (profileId)
    B->>S: Connect
    S-->>B: connection.ready (profileId)
    W->>S: game.create (White)
    S-->>W: game.created (gameId, White)
    Note over W,B: Share the game ID
    B->>S: game.join (gameId)
    Note over S: Start White's clock
    S-->>B: game.joined (Black)
    S-->>B: game.initial (version 0)
    S-->>W: game.initial (version 0)
    W->>S: game.move (e2e4, expectedVersion 0)
    Note over S: Apply rules and update the clock and timer
    S-->>W: game.state (version 1)
    S-->>B: game.state (version 1)
    Note over W,B: Continue with alternating moves and authoritative updates
    B->>S: game.resign
    S-->>B: game.state (finished)
    S-->>W: game.state (finished)
    Note over W,B: Both sessions can create, join, or match again
```

## Terminal client

`make chessli` builds and runs the server. PostgreSQL is required, and startup
checks the connection. Set `DATABASE_URL` in `.env` using `.env.example` as a
template, then run `make run-deps` and `make seed-local`. Run `make generate`
explicitly when generated code needs updating. An existing database must be
recreated to pick up changes to the initial migration.

Start the server with `make run`, then run the following commands in two other
terminals, using the seeded development profiles:

```sh
make run-tui PROFILE_ID=00000000-0000-4000-8000-000000000001
make run-tui PROFILE_ID=00000000-0000-4000-8000-000000000002
```

The TUI connects to `ws://localhost:8080/ws` and displays its profile ID separately
from the mocked dashboard profile. Connections require an existing profile's UUID;
registration and login are separate future work. The server supplies any unfinished
game's ID, and the TUI automatically resumes it.
Use a terminal of at least 80 columns by 24 rows; smaller terminals show a
resize prompt instead of a clipped board. Dashboard sample data is labeled
in the client, and History shows an unavailable state.

Press Tab to open Play Chess. Type `/create` and press Enter in the first
client; it creates a 10+0 private game as White. Copy the displayed game ID,
then type `/join GAME_ID` in the second client and press Enter. The first
client waits until the second joins before displaying a board.

Alternatively, enter `/match` in both clients to find an opponent in the 10+0
pool. Admission clears the submitted command only if you have not edited it.
Waiting for an opponent has no deadline. Assignment supplies your game ID and
color; moves and resignation stay blocked until `game.initial` arrives. An
initially offline opponent can become online in a same-version presence update.

Enter SAN moves such as `e4`, `Nf3`, or `O-O`, or UCI moves such as `e2e4` and
`a7a8q`. Use `/resign` to finish a game. A finished game is read-only; use
`/match`, `/create` or `/join GAME_ID` to play another. The plain board comes from the
existing chess library and shows only server-confirmed positions. Clocks show
the last received server values, rather than a local countdown.

Updates continue on Dashboard and History; navigating away does not resign or
stop server clocks. Session moves show the latest received SAN moves, with
version labels and gaps when updates were missed. The display shows the last
four entries; it is not a complete history, especially after restarting the client.
Dashboard statistics and History remain mocked or placeholders.

Escape clears the current input before returning to Dashboard. Ordinary text
keys, including `q`, go to the Play input. Ctrl+C quits; `q` also quits outside
Play.
Server replies preserve text edited while a request was pending. Long input
shows its ending and cursor; the full value is submitted.
There is no automatic reconnect or saved identity. Restart with the same profile
ID to recover an unfinished game; a server restart loses that game.

Only one action can be pending. A socket write does not acknowledge an action:
the TUI waits for the server response or initial state. Missing responses and
write failures show an unknown outcome and block further submissions until
recovery. Restart with the same profile ID to recover rather than replay actions,
including when a create or matchmaking assignment response was lost. A stale-version
rejection also requires recovery before another move.

Disconnect cancels a waiting ticket; reopening the client never automatically
requeues. If pairing committed before disconnect, startup supplies the game ID
even if the assignment response was lost. The [manual checks](docs/manual-error-testing.md#terminal-client-checks)
cover delayed replies, recovery, and malformed server messages.

## Manual two-player flow

This flow requires [`websocat`](https://github.com/vi/websocat).

Start the server from the repository root:

```sh
make run
```

`GET /healthz` and `GET /readyz` return `200 OK` once the server is listening.
Startup checks PostgreSQL; `readyz` does not continuously probe the database.

Open two more terminals and connect both players:

```sh
websocat 'ws://localhost:8080/ws?profileId=00000000-0000-4000-8000-000000000001'
websocat 'ws://localhost:8080/ws?profileId=00000000-0000-4000-8000-000000000002'
```

Each connection receives a `connection.ready` `profileId` and an optional
`activeGameId`. Reconnect with `?profileId=PROFILE_ID`, then send `game.resume`
for that game. The TUI does this automatically. The profile must exist in the
database; supplying its ID is still development-only identification, without
authentication. A profile may have only one unfinished game across private games
and matchmaking, including across connections.

`connection.ready` is always sent before commands are processed. Its write has
a 10-second deadline and is canceled when the server shuts down.

In the first player terminal, create a game as White:

```json
{"type":"game.create","requestId":"create-1","payload":{"color":"white","timeControl":"10+0"}}
```

Copy the `gameId` from the `game.created` response. In the second player
terminal, replace `GAME_ID` with that value and join the game:

```json
{"type":"game.join","requestId":"join-1","payload":{"gameId":"GAME_ID"}}
```

Both players should receive a `game.initial` message with version `0`.
Each `white` and `black` player object contains `profileId`, `color`,
`connected`, `remainingMilliseconds`, and the player's preferred `notation`.
Notation is currently fixed to `uci` until player preferences are configurable.
When a player disconnects or resumes, the opponent receives an unsolicited
`game.state` with the corresponding `connected` value updated. The server also
uses native WebSocket ping/pong to detect unresponsive connections.

In the first player terminal, play `e2e4`:

```json
{"type":"game.move","requestId":"move-1","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci","expectedVersion":0}}
```

Both players should receive the updated state with version `1`. In the second
player terminal, play `e7e5` using that version:

```json
{"type":"game.move","requestId":"move-2","payload":{"gameId":"GAME_ID","move":"e7e5","notation":"uci","expectedVersion":1}}
```

Both players should now receive version `2`, with the authoritative FEN and
the last move represented as SAN.

Either player can offer a draw without changing the game version:

```json
{"type":"draw.offer","requestId":"draw-1","payload":{"gameId":"GAME_ID"}}
```

The resulting `game.state` contains `pendingDrawOffer` with an opaque `offerId`
and the offering color. The opponent can accept or decline that exact offer:

```json
{"type":"draw.accept","requestId":"draw-2","payload":{"gameId":"GAME_ID","offerId":"OFFER_ID"}}
{"type":"draw.decline","requestId":"draw-3","payload":{"gameId":"GAME_ID","offerId":"OFFER_ID"}}
```

Declining leaves the version unchanged. Accepting increments it and finishes
the game with reason `draw_agreement`. A successful move clears a pending
offer, and each player must wait two minutes before offering again.

A player can resign without supplying a game version:

```json
{"type":"game.resign","requestId":"resign-1","payload":{"gameId":"GAME_ID"}}
```

When a move finishes the game, the same `game.state` has status `finished` and
includes an `outcome` with the result and reason. The currently reachable
reasons are `checkmate`, `stalemate`, `timeout`, `fivefold_repetition`,
`seventy_five_move_rule`, `insufficient_material`, `resignation`, and
`draw_agreement`. Threefold and fifty-move claims are not implemented yet.
After the final state is queued for delivery, both live connections are free to
create, join, or match into another game.
The server attempts delivery to the opponent even if the acting player's
connection has stopped, its queue is full, or its request was canceled.
This also applies to committed join and presence updates during resume and
matchmaking, even when the acting player's acknowledgement cannot be queued.
State publication and admission are ordered so a delayed operation cannot
publish active state after a finished state or occupy a finished game.
Delivery remains best effort: queue acceptance does not confirm receipt.
A former participant can use `game.resume` to load a finished game's final
state. That view is read-only and does not mark either player connected.

Clocks are authoritative on the server. When the active player's time reaches
zero, the server automatically increments the state version and broadcasts a
finished `game.state` with reason `timeout`. If the opponent has only a king,
the timeout result is a draw because that player cannot checkmate. A move
racing with the clock is not applied once that player's time has expired.

## Manual matchmaking flow

Start the server and connect two terminals as described above. In both player
terminals, enter the same matchmaking pool:

```json
{"type":"matchmaking.enter","requestId":"match-1","payload":{"timeControl":"10+0"}}
```

Each player first receives `matchmaking.entered` with the entry `requestId` and
`{"timeControl":"10+0"}` payload. Once both players are queued,
each receives `matchmaking.found` containing the shared `gameId` and that
player's assigned `color`, using that same `requestId`, followed by an
uncorrelated authoritative `game.initial`. Waiting for an opponent has no deadline.
The second entry publishes both assignments under the handler's dispatch lock,
registering the waiting player before the arriving player. If the
first initial state shows the opponent as disconnected, a subsequent
unsolicited `game.state` reports that opponent's registration.

In whichever terminal was assigned White, replace `GAME_ID` and play the first
move:

```json
{"type":"game.move","requestId":"move-1","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci","expectedVersion":0}}
```

In the terminal assigned Black, use the same `gameId` and the version from the
latest `game.state` response:

```json
{"type":"game.move","requestId":"move-2","payload":{"gameId":"GAME_ID","move":"e7e5","notation":"uci","expectedVersion":1}}
```

Supported matchmaking presets are `1+0`, `1+1`, `2+1`, `3+0`, `3+2`, `5+0`,
`10+0`, `10+5`, `15+10`, and `30+0`. Players only match when they select the
same preset.

Opening another connection with the same `?profileId=PROFILE_ID` does not
bypass gameplay checks: joining a game that already contains that profile
returns `invalid_message` / `already a participant in this game`, and entering
matchmaking while that profile is already queued returns `invalid_message` /
`already queued for matchmaking`.

For payloads that exercise the current error responses, see
[Manual WebSocket Error Testing](docs/manual-error-testing.md).

Malformed JSON receives an `invalid_message` error and does not close the
connection, so the client can correct and resend its request. A `game.move`
payload must also include a non-empty `gameId`.

## Operational logs

The existing logger writes JSON to stdout by default. Set `CHESSLI_LOG_FORMAT=text`
for local reading. Filter on `event`, not the human-readable `msg`.

| Event | Meaning and useful fields |
| --- | --- |
| `server.starting`, `server.stopped` | Process lifecycle; startup includes `address`. |
| `connection.opened`, `connection.closed` | Upgraded socket lifetime, with `profile_id`; close includes `duration_ms`, an available `close_code`, and `reason` (`client_closed`, `transport_closed`, `shutdown`, `timeout`, or `error`). |
| `game.created` | A private game is waiting for its opponent; includes `game_id`, creator `profile_id`, `initial_ms`, and `increment_ms`. |
| `game.started` | Both seats are filled; includes `game_id`, `mode` (`private` or `matchmaking`), both profile IDs, and the time control. |
| `matchmaking.queued`, `matchmaking.canceled` | A waiting player's queue lifetime, correlated by `profile_id`; queue entry includes the time control. An immediate match emits `game.started`. |
| `game.resumed` | A participant successfully reads their game through resume; includes `game_id`, `profile_id`, `outcome`, and `version`. This does not confirm client receipt. |
| `game.player_disconnected` | A connection is removed from its game; includes `game_id`, `profile_id`, and `server_shutdown` to distinguish server shutdown from an individual disconnect. |
| `game.finished` | One completion event per game transition, with `game_id`, `outcome`, `termination`, and final `version`. Reads and reconnects do not repeat it. |
| `message.dropped` | A full outgoing queue; includes `profile_id`, `request_id`, `message_type`, and `reason=queue_full`. |
| `game.persisted` | Completed game, moves, and final clocks saved to PostgreSQL. |
| `game.persistence_retry`, `game.persistence_failed` | A save is being retried, or all three attempts failed; includes `game_id` and the error. |

Lifecycle events use INFO. Full queues, rejected upgrades (`connection.rejected`),
timeouts, and unexpected disconnects use WARN. Unexpected application failures
use ERROR (`request.failed`, `disconnect.state_failed`, `server.failed`, and
`server.configuration_failed`), with the error and available correlation IDs.
EOF disconnects use INFO with `reason=transport_closed`; routine closes omit the
error detail, and `close_code` is omitted when no WebSocket close status is available.

Routine moves, heartbeat traffic, and ordinary client validation errors are not
logged. Message payloads are not included. Use these events for operational
dashboards; they do not provide durable game history or delivery acknowledgements.

## Repository integration tests

The application saves completed games asynchronously. Each save has up to three
attempts, with a three-second timeout per attempt and one second between attempts.
Shutdown waits for these writes before closing the pool. Failed saves are logged;
pending records have no crash recovery. Stored games survive restart, while active
games, clocks, and matchmaking remain in memory. Dashboard and History are still
placeholders; stored history is available through the repository methods.

Run repository integration tests with a test database:

```sh
CHESSLI_TEST_DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable' make test-integration
```

Use a test database whose user can create schemas. Each test creates a unique
schema, applies the migrations, and drops the schema and records at cleanup.
CI runs these tests against PostgreSQL 17 in its integration job. See
[test suite organization](docs/adr/0001-test-suite-organization.md) for the unit,
integration, and stack conventions and commands.

The initial migration now stores moves inside completed games. Databases created
from the previous schema need to be recreated to use it. For disposable local
data, `make reset-local-db` recreates the database, applies migrations, and seeds it.
