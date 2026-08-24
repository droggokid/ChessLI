# Manual WebSocket Error Testing

This guide contains messages that can be pasted into `websocat` to exercise
the server's current error handling.

Start the server:

```sh
make run
```

Open a terminal for each required client:

```sh
websocat ws://localhost:8080/ws
```

Each WebSocket connection receives its own profile ID and may participate in
only one game. Open a fresh connection or restart the server when a scenario
requires a clean game.

The expected-error examples below show the `payload` portion of the server's
`error` envelope.

## Common setup

In client A, create a game as White:

```json
{"type":"game.create","requestId":"setup-create","payload":{"color":"white","timeControl":{"initialMilliseconds":600000,"incrementMilliseconds":0}}}
```

Copy the returned game ID and replace `GAME_ID` in the examples below.

When a scenario requires an active game, join from client B:

```json
{"type":"game.join","requestId":"setup-join","payload":{"gameId":"GAME_ID"}}
```

## Reachable gameplay errors

### Game not found

This can be sent from any client:

```json
{"type":"game.join","requestId":"error-game-not-found","payload":{"gameId":"missing-game"}}
```

Expected error:

```json
{"code":"game_not_found","message":"game not found"}
```

### Game not ready

After client A creates a game, attempt a move before client B joins:

```json
{"type":"game.move","requestId":"error-game-not-ready","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci","expectedVersion":0}}
```

Expected error:

```json
{"code":"illegal_move","message":"game is waiting for another player"}
```

### Game full

After clients A and B occupy both seats, connect client C and send:

```json
{"type":"game.join","requestId":"error-game-full","payload":{"gameId":"GAME_ID"}}
```

Expected error:

```json
{"code":"game_full","message":"game is full"}
```

### Not your turn

At version `0`, send a Black move from client B before White has moved:

```json
{"type":"game.move","requestId":"error-not-your-turn","payload":{"gameId":"GAME_ID","move":"e7e5","notation":"uci","expectedVersion":0}}
```

Expected error:

```json
{"code":"not_your_turn","message":"not your turn"}
```

### Illegal move

At version `0`, send an illegal move from client A:

```json
{"type":"game.move","requestId":"error-illegal-move","payload":{"gameId":"GAME_ID","move":"e2e5","notation":"uci","expectedVersion":0}}
```

Expected error:

```json
{"code":"illegal_move","message":"illegal move"}
```

### Stale game version

Send a move with a version that does not match the latest `game.state`:

```json
{"type":"game.move","requestId":"error-stale-version","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci","expectedVersion":99}}
```

Expected error:

```json
{"code":"stale_game_version","message":"game state is stale"}
```

### Game finished

On a fresh active game, play Fool's Mate in this order:

Client A:

```json
{"type":"game.move","requestId":"finish-1","payload":{"gameId":"GAME_ID","move":"f2f3","notation":"uci","expectedVersion":0}}
```

Client B:

```json
{"type":"game.move","requestId":"finish-2","payload":{"gameId":"GAME_ID","move":"e7e5","notation":"uci","expectedVersion":1}}
```

Client A:

```json
{"type":"game.move","requestId":"finish-3","payload":{"gameId":"GAME_ID","move":"g2g4","notation":"uci","expectedVersion":2}}
```

Client B:

```json
{"type":"game.move","requestId":"finish-4","payload":{"gameId":"GAME_ID","move":"d8h4","notation":"uci","expectedVersion":3}}
```

After the checkmate produces version `4`, attempt another move from client A:

```json
{"type":"game.move","requestId":"error-game-finished","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci","expectedVersion":4}}
```

Expected error:

```json
{"code":"game_finished","message":"game is finished"}
```

### Invalid color preference

Send from a client that is not already registered with a game:

```json
{"type":"game.create","requestId":"error-color","payload":{"color":"purple","timeControl":{"initialMilliseconds":600000,"incrementMilliseconds":0}}}
```

Expected error:

```json
{"code":"invalid_message","message":"invalid color preference"}
```

### Invalid time control

```json
{"type":"game.create","requestId":"error-time-control","payload":{"color":"white","timeControl":{"initialMilliseconds":0,"incrementMilliseconds":0}}}
```

Expected error:

```json
{"code":"invalid_message","message":"invalid time control"}
```

### Unsupported move notation

```json
{"type":"game.move","requestId":"error-notation","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"pgn","expectedVersion":0}}
```

Expected error:

```json
{"code":"invalid_message","message":"unsupported move notation"}
```

## Handler and protocol errors

### Invalid JSON

```text
{"type":"game.join"
```

Expected error:

```json
{"code":"invalid_message","message":"message must be valid JSON"}
```

### Unknown message type

```json
{"type":"game.unknown","requestId":"error-unknown-type","payload":{}}
```

Expected error:

```json
{"code":"unknown_message_type","message":"unknown message type \"game.unknown\""}
```

### Invalid payload

Unknown payload fields are rejected:

```json
{"type":"game.join","requestId":"error-invalid-payload","payload":{"gameId":"GAME_ID","unexpected":true}}
```

Expected error:

```json
{"code":"invalid_message","message":"invalid game.join payload"}
```

### Missing game ID

```json
{"type":"game.join","requestId":"error-missing-game-id","payload":{"gameId":""}}
```

Expected error:

```json
{"code":"invalid_message","message":"gameId is required"}
```

### Missing move

```json
{"type":"game.move","requestId":"error-missing-move","payload":{"gameId":"GAME_ID","move":"","notation":"uci","expectedVersion":0}}
```

Expected error:

```json
{"code":"invalid_message","message":"move is required"}
```

### Missing expected version

```json
{"type":"game.move","requestId":"error-missing-version","payload":{"gameId":"GAME_ID","move":"e2e4","notation":"uci"}}
```

Expected error:

```json
{"code":"invalid_message","message":"expectedVersion is required"}
```

### Session already in a game

After a client creates or joins a game, send another create command from that
same connection:

```json
{"type":"game.create","requestId":"error-session-game","payload":{"color":"white","timeControl":{"initialMilliseconds":600000,"incrementMilliseconds":0}}}
```

Expected error:

```json
{"code":"invalid_message","message":"session is already in a game"}
```

### Not implemented

```json
{"type":"game.resign","requestId":"error-not-implemented","payload":{}}
```

Expected error:

```json
{"code":"not_implemented","message":"game.resign is not implemented"}
```

The matchmaking and draw message types currently produce the same
`not_implemented` code.

## Errors that are not currently reachable through WebSocket payloads

The following gameplay errors exist in `mapApplicationError`, but the current
handlers cannot produce them:

- `ErrNotParticipant`: move validation currently reports `ErrNotYourTurn` when
  a non-participant submits a move.
- `ErrAlreadyParticipant`: an existing session is rejected by `GameSessions`
  before the gameplay join method is called, while a new connection receives a
  new profile ID.
- `ErrAlreadyQueued`: matchmaking currently returns `not_implemented` without
  calling the gameplay service.
- `ErrNoCompatibleOpponent`: matchmaking currently returns `not_implemented`
  without calling the gameplay service.
- The default `internal_error` mapping has no intentional client payload that
  triggers it; it is a fallback for unexpected server errors.

These entries should receive payload examples when their corresponding
WebSocket paths become reachable.
