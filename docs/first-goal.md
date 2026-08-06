# First Goal

Deliver a standard chess game for two players in separate terminals, with the
server as the authoritative source of game state.

## WebSocket Transport

- Upgrade connections at `/ws`.
- Send and receive typed JSON envelopes.
- Return structured errCh for invalid messages.
- Handle connection, disconnection, Ping/Pong, and slow-client cleanup.
- Do not broadcast raw messages globally.

## One-Game Slice

- Create one game with a White player and a Black player.
- Let both players join with a game ID.
- Accept UCI move input, such as `e2e4`.
- Validate player, turn, legality, and expected game version on the server.
- Apply legal moves and broadcast the resulting FEN, SAN, and game state to
  both players.
- Reject illegal, duplicate, stale, and wrong-player moves.

## Definition Of Done

Start the server, open two terminals, play `e2e4 e7e5`, and observe both
clients update from the server's authoritative state.

## Out Of Scope

Do not build Kafka, GraphQL, ratings, tournaments, bots, or generic chat
before this slice works. Use in-memory state for the WebSocket transport, then
add PostgreSQL for the durable one-game slice.
