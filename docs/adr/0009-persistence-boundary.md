# ADR 0009: Persistence Boundary

Status: Accepted

## Decision

Use PostgreSQL for durable application data and `sqlc` for database access.

The database is authoritative for profiles and completed games. Live game state,
clocks, matchmaking, and WebSocket sessions remain in memory. A process restart
therefore does not recover unfinished games in the first version.

Persist completed games as immutable records containing their players, time
control, terminal outcome, termination reason, final position, and move history.
Derive player matchup history from those records; do not store a duplicate
matchup-history table.

Do not introduce a generic repository. Each concrete persistence adapter owns
one narrow interface as its mock seam and public storage contract, such as a
profile store or completed-game store. Keep generated `sqlc` types and query
objects inside the adapter; do not embed generated queries.

Keep the adapter interface and its generated GoMock mock in the adapter
package. Regenerate the mock from a colocated `go:generate` directive whenever
the interface changes. Adapter APIs use domain identifiers and models; UUID and
`sqlc` values are mapped at the adapter boundary.

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
- Persistence interfaces and their generated mocks change together; query and
  driver types remain implementation details.
