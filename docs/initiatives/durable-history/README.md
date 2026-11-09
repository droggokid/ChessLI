# Initiative: durable-history

Local initiative overview; intentionally excluded from version control.
Feature implementation plans are stored under this initiative's `plans/` directory.

## Outcome

Retain complete game records and establish identities suitable for later durable
gameplay storage. Personal History, imports, and analysis require registration;
registration/login are a separate initiative. The original full-history acceptance
test therefore depends on that initiative. Unfinished games remain in memory.

Current iteration: [persistent play identities and completed-game storage
implementation plan](plans/02-persistent-play-storage.md). Guest identity is part
of this iteration, not a separate initiative. The user requested an implementation
handoff; this source task only prepares the plan structure and handoff.

## Working approach

Use the existing `feature/durable-history` branch for this initiative. Keep
related increments together; do not create a development branch per small task.
Finish the user-visible outcome before moving to the next initiative.

## Increments

1. **Complete game records.** Retain the original time control, ordered UCI moves,
   and both post-move clock balances. Cover all terminal outcomes and preserve
   existing move, timeout, and snapshot behavior. Begin here; database wiring
   follows after the record is complete.
2. **Durable profiles and completion saves.** Implement persistent guest
   identity, an environment-gated development bypass and completed-game saves
   together using the reviewed feature plan.
   Development retains random IDs per connection and optional explicit UUIDs for
   reconnect tests, without requiring named profiles or a local identity file. Outside
   development, guests use stored private credentials. Guests can play anyone.
   Both environments persist profiles and completed games through PostgreSQL;
   the development switch affects identity authentication, not persistence.
   Registration gating for personal libraries is settled. The feature plan records
   the proposed same-profile connection policy and other implementation defaults.
   Save each finished game as one immutable record, with database I/O outside the
   gameplay mutex. Return save errors and log them at the application boundary;
   preserve result delivery. Reliability policies, retries and queues are outside
   this feature's scope.
3. **Usable History (requires registration).** Expose paginated history for the selected registered identity, show
   opponent/result/completion time, and open a game to review its moves. Replace
   the History placeholder with real data; extend dashboard statistics afterward.

## Decisions before implementation

### Increment one implementation

Work in the existing `feature/durable-history` initiative branch and saved checkout.
Retain initial/increment durations on the game, append both clock balances after
each successful engine move and clock update, and stamp completion at the terminal
transition (the deadline for timeout). Reuse the engine's ordered moves and encode
UCI during extraction; only clock history is stored alongside it. Keep detached
completed-record extraction package-private until persistence wiring supplies an
external caller. Database tags and storage DTOs stay in persistence packages.

- [x] Add focused record behavior tests; extend existing rejection tests to assert
  no move is recorded. Use controlled time for move balances and stable completion.
- [x] Retain time control, moves, and completion time; copy moves when extracting.
  Cover engine completion, resignation, agreed draw, and timeout without changing
  snapshots, versioning, scheduling, or publication.
- [x] Run `go test ./...`, `go vet ./...`, and `go test -race ./...`; review the diff.
  Leave changes uncommitted. Database wiring and UI remain later increments.

Verification: all three commands passed on 2026-11-09. Full test/vet runs needed
approval for the default Go build cache outside the sandbox. Read-only independent
review found no issues. Increment one is complete in the working tree; no commit,
database wiring, public history API, or UI changes. Custom starting-position replay
remains with the deferred import work; live games currently start in the standard
position.

- Identity policy: development bypass versus persistent guests outside dev and
  database persistence in both environments are agreed. The proposed default
  environment, same-profile connection policy and expiry deferral are implementation
  defaults proposed in the feature plan, not separately confirmed user choices.
  The existing schema must support guests without account fields.
- Guest history/library, imports, and analysis are registration-only. Do not add
  guest library access or guest analysis/import compute.
- History API: GraphQL is the roadmap preference; choose the smallest concrete
  query slice and tooling when the read API is implemented.
- Memory retirement can be considered later; save reliability/retry policy is
  deliberately outside the current feature plan.

## Boundaries and acceptance

Keep named gameplay operations and the current synchronization model. No imports,
engine analysis, ratings, active-game restart recovery, or unrelated concurrency
rewrite in this initiative. Preserve normal live play when persistence fails.

Use behavior tests for recorded moves and completion, PostgreSQL integration
tests for saves/queries, and the race detector for concurrency changes. This
iteration's manual check uses independent dev clients, completed-game persistence
and database reload after restart. Registered History verification follows the
registration initiative. Each increment should leave the app runnable.
