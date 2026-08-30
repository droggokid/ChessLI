# ChessLI
ChessLI is a terminal-first chess platform inspired by Chess.com, built for playing games, managing matches, and exploring chess from the command line.

## Manual two-player flow

This flow requires [`websocat`](https://github.com/vi/websocat).

Start the server from the repository root:

```sh
make run
```

Open two more terminals and connect both players:

```sh
websocat ws://localhost:8080/ws
```

Each connection should receive a `connection.ready` message.

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

When a move finishes the game, the same `game.state` has status `finished` and
includes an `outcome` with the result and reason. The currently reachable
reasons are `checkmate`, `stalemate`, `timeout`, `fivefold_repetition`,
`seventy_five_move_rule`, and `insufficient_material`. Resignation, draw
agreement, threefold claims, and fifty-move claims are not implemented yet.

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

Each player first receives `matchmaking.entered`. Once both players are queued,
each receives `matchmaking.found` containing the shared `gameId` and that
player's assigned `color`, followed by the authoritative `game.initial` state.

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

For payloads that exercise the current error responses, see
[Manual WebSocket Error Testing](docs/manual-error-testing.md).
