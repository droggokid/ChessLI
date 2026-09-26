# ADR 0009: Persistence Boundary

Status: Proposed

## Decision

Use PostgreSQL for durable application data and `sqlc` for database access.

The database is authoritative for profiles and completed games. Live game state,
clocks, matchmaking, and WebSocket sessions remain in memory. A process restart
therefore does not recover unfinished games in the first version.

Persist completed games as immutable records containing their players, time
control, terminal outcome, termination reason, final position, and move history.
Derive player matchup history from those records; do not store a duplicate
matchup-history table.

Do not introduce a generic repository. When an application consumer needs one,
define a narrow, consumer-owned interface such as a completed-game store or
profile store. Keep generated `sqlc` types inside the storage implementation.

## Rationale

The first queries are known in advance: create/read profiles, save a completed
game, list a player's games, and load one game. `sqlc` keeps those queries as
reviewable SQL and generates typed Go calls. Squirrel primarily helps compose
dynamic SQL and is unnecessary here.

## Consequences

- A database outage must never undo a completed game or prevent its final state
  from reaching connected players; initial persistence failures are logged.
- Reliable retry and active-game recovery are later, separate decisions.
- Friendship semantics and preference fields are deferred until their product
  requirements are defined.
