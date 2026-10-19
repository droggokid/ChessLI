# ADR 0005: Persistence Boundary

Status: Accepted.

## Context and decision

Use PostgreSQL for durable application data and `sqlc` for typed database
access. The database is the intended authority for profiles and completed
games. Live games, clocks, matchmaking, and sessions stay in memory; a restart
does not recover unfinished games.

Store completed games as immutable records of players, time control, terminal
outcome, termination reason, final position, and move history. Derive player
matchup history from those records rather than duplicating it in another table.

Storage operations use domain identifiers and models; driver and generated
query representations are translated at the adapter boundary. Interface
ownership, mock generation, and internal adapter layout are implementation
choices.

A database outage must not undo game completion or prevent final-state
publication to connected players. Initial persistence failures are logged;
reliable retry and active-game recovery remain separate future decisions.
Publication still has the delivery limits in
[ADR 0003](0003-message-delivery-semantics.md).

## Rationale and trade-offs

The initial queries are known: create/read profiles, save completed games,
list a player's games, and load a game. Reviewable SQL and generated types fit
these needs without a generic repository or dynamic query builder. Keeping
live play in memory simplifies clocks and matchmaking, at the cost of restart
loss and no guaranteed durable completion during an outage.

## Implementation status

The schema, queries, and adapters exist, but `cmd/main.go` does not yet wire
persistence into gameplay. WebSocket profile IDs remain placeholder identities.
Completed-game and move writes are currently separate operations, not an atomic
save of the whole record. This decision describes the storage boundary and
failure policy; it does not claim those integration or durability gaps are
already solved.
