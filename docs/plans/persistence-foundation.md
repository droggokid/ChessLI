# Persistence Foundation Plan

## Goal

Add durable player identities and completed-game history without changing the
in-memory ownership of live games.

## Scope

- PostgreSQL, migrations, and `sqlc` generation.
- Durable profile rows keyed by the existing `ProfileID`.
- Immutable completed-game records with players, time control, outcome,
  termination reason, final FEN, and move history.
- Queries to save a completed game, list a profile's games, and load one game.

## Out of Scope

- Active-game recovery after restart.
- A generic repository or persistence-backed `gameplay.Service`.
- Friendship workflows and preference fields: their semantics are not defined.
- Ratings, rematches, spectators, and analysis features.

## Steps

1. Choose the PostgreSQL driver and migration tool; add a pinned `sqlc` tool
   directive and `sqlc.yaml`.
2. Create the initial schema and migrations for `profiles`, `completed_games`,
   and `completed_game_moves`. Add only the indexes required by the planned
   profile-history queries.
3. Write named SQL queries, generate the storage package, and test migrations
   plus the generated-query integration against PostgreSQL.
4. Add small storage adapters at the consumers that need them. Do not expose
   generated types through gameplay or WebSocket APIs.
5. At terminal transition, build a completed-game record and attempt to save
   it before releasing transport membership. If saving fails, log the failure
   and still publish the final state; do not make players retry a completed game.
6. Add history retrieval only after the stored record and authorization policy
   are settled.

## Acceptance Criteria

- Creating or reusing a profile yields a durable profile row.
- A completed game is queryable after a server restart.
- A database failure cannot reverse or hide a terminal game result.
- Unfinished games remain intentionally unrecoverable after restart.

## Later Decisions

- Define friend-request/blocking behavior before adding friendship tables.
- Define preference fields before adding a preferences table.
- Decide whether durable active-game events or snapshots are needed for restart
  recovery.

## Future: Imported Games and Dashboard Insights

After completed-game history is stable, accept PGN uploads into the same
canonical completed-game model as ChessLI games. Record a game's source,
external ID when present, and original move sequence.

Direct Lichess and Chess.com imports come after PGN upload and require an
account-linking and rate-limit design. Derive matchup history and opening
frequency from stored games; do not duplicate either as primary data.

Run chess-engine analysis asynchronously after a game is stored. Keep derived
evaluations, mistakes, and engine settings separate from the immutable game
record. The first dashboard insight slice is game history, result summaries,
and opening frequency; engine insights are later.
