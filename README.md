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
{"type":"game.create","requestId":"create-1","payload":{"color":"white","timeControl":{"initialMilliseconds":600000,"incrementMilliseconds":0}}}
```

Copy the `gameId` from the `game.created` response. In the second player
terminal, replace `GAME_ID` with that value and join the game:

```json
{"type":"game.join","requestId":"join-1","payload":{"gameId":"GAME_ID"}}
```

Both players should receive a `game.state` message with version `0`.

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

For payloads that exercise the current error responses, see
[Manual WebSocket Error Testing](docs/manual-error-testing.md).
