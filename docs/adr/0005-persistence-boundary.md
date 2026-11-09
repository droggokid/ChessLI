# ADR 0005: Persistence Boundary

Status: Accepted.

## Decision

Use PostgreSQL, pgx pools, and Squirrel query builders for durable profiles and
completed games. Live games, clocks, and matchmaking remain in memory; a restart
does not recover unfinished play.

`internal/persistence/game` and `internal/persistence/profile` each own a
`Repository`, `RepositoryImpl`, generated mock, and storage models. Driver and
JSON representations stay inside those packages.

A completed-game row stores players, time control, terminal outcome/reason,
final position, completion time, and an ordered JSONB moves array. Each move
includes its ply, UCI notation, and both remaining clocks in milliseconds.
`CreateGame` saves the whole record in one insert; `GetGameByID` loads it with
its moves. Constraints validate players, time controls, outcomes, and moves.
Player history is derived from these records.

## Rationale and current status

Squirrel keeps parameterized SQL reviewable without generated query code.
Moves share the immutable game's lifetime, so embedding them makes saving atomic.
Per-move queries are less direct; current queries load whole games or summaries.

The schema, seed, repositories, and PostgreSQL integration tests exist.
Application wiring, authentication, completion persistence, and retry are still
future work; the running server uses placeholder identities and memory only.
When storage is wired in, failures must not undo game completion or prevent
final-state delivery. See the [roadmap](../roadmap.md).

The initial migration reflects this layout. Existing development databases need
recreation to adopt it; editing an applied migration does not rerun it.
