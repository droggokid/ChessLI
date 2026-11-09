# Implementation plan: persistent play identities and completed-game storage

Iteration 2 feature plan under the existing [durable-history initiative](../README.md).
Guest identity is one part of this iteration, not an initiative of its own.
The user requested a handoff for implementation on 2026-11-09. This source task
does not implement the feature. Remain on `feature/durable-history`; preserve the
completed, uncommitted first increment. Plans stay local and ignored by Git.

**Goal:** Persist gameplay profiles and complete game records in development and
production, with a development identity bypass and persistent production guests.

**Architecture:** PostgreSQL stores profiles and completed games in both modes.
Environment affects identity admission, never whether data is saved. Gameplay
owns the terminal transition and emits a detached completed record; application
wiring translates it to storage models and performs asynchronous I/O.
Personal-library features remain registration-only.

**Execution:** In the implementation destination, use inline execution with the executing-plans and
test-driven-development skills, then one read-only code review. No commits or
publishing unless requested.

## Agreed behavior

- `CHESSLI_ENV=dev`: a connection without an explicit ID receives a random UUID.
  An explicit `profileId` / CLI `-profile-id` remains available for reconnect and
  fixture testing. Four terminals can represent four independent players; there
  is no named-profile feature, fixed profile count, or local credential file in dev.
- Dev identities are real database participants. Before gameplay admission,
  idempotently ensure the chosen profile exists. Preserve existing test/account
  rows when an explicit UUID is reused; do not overwrite them with guest data.
- Outside dev, bootstrap a persistent guest profile and private token once, then
  reuse the locally saved credential. A public ProfileID is not authentication.
  The server, not the client's environment, decides whether the bypass is allowed.
- Both environments require PostgreSQL and persist completed games. Missing
  database configuration or failure to establish the database connection prevents
  startup. There is no silent in-memory fallback.
- Use ordinary error handling for completion saves: the persistence function
  returns its error and the application caller logs it with the game ID. Do not
  design save guarantees, retries, recovery queues or a special timeout policy
  for this feature.
- Guests can play any supported participant. Registration is unnecessary for
  normal play. Personal history/library, imports, analysis and their compute
  require registration. Saving the shared game record grants no library access.
- Production currently uses one automatically saved identity per installation.
  Manually testing multiple independent production identities is not a requirement.
- The credential file contains only the ProfileID and token, with no server
  address. Connection configuration remains separate.
- Registration/login, external ratings, public history APIs/UI, game retention,
  guest deletion and unfinished-game recovery are not implemented by this feature.
  Existing live-game resume and final snapshots remain available to participants.

## Implementation defaults proposed during planning

1. **Environment:** accept `dev` and `prod`, default to `prod` when unset, and
   reject unknown values. Both binaries read this setting; only the server can
   authorize a bypass. Existing local setup examples explicitly select dev.
2. **Same profile in two clients:** allow one accepted WebSocket per profile in
   both modes. Reject a duplicate with HTTP 409 before upgrading, with a clear
   client-facing message. Do not replace the first connection or create another
   identity automatically. Close the first client before reopening that profile.
3. **Expiry:** defer enforcement because no duration has been agreed. Do not
   assume 30/90 days or conflate access expiry with deleting retained game rows.

## Task 1 — Store guest and development profiles

**Files:** new additive migration under `internal/database/migrations/`;
`internal/persistence/profile/repository.go`, `profile.go`, generated mock;
`internal/persistence/repository_integration_test.go`;
new `internal/guest/service.go` and focused service tests.

**Interfaces:** the guest application service consumes a narrow store implemented
by the profile repository:

```go
CreateGuest(context.Context, [32]byte) (identity.ProfileID, error)
GuestProfileID(context.Context, [32]byte) (identity.ProfileID, error)
EnsureDevProfile(context.Context, identity.ProfileID) error
```

An unknown digest returns an empty ID without error. `guest.Service` offers
`Create(ctx) (Credential, error)`, `Authenticate(ctx, token) (ProfileID, error)`
and `EnsureDevProfile(ctx, id) error`; `Credential` contains ProfileID and Token.
Invalid credentials and database failures remain distinct application errors.

- [ ] Write failing tests for distinct guest credentials, authentication, invalid
  tokens, idempotent dev UUID reuse, and database failure propagation.
- [ ] Add a migration permitting guests without email/password and a nullable
  unique 32-byte guest token digest. Guests may have no digest for development;
  production bootstrap always creates one. Keep a generated unique guest username
  and existing account constraints; no fabricated emails, passwords or dev tokens.
- [ ] Implement cryptographically random 256-bit guest secrets, storing only their
  SHA-256 digests. `EnsureDevProfile` inserts a missing UUID without modifying an
  existing row. Production cannot authenticate a credentialless dev profile.
- [ ] Preserve account repository behavior and completed-game participant foreign
  keys. Map nullable guest fields appropriately and regenerate affected mocks.
- [ ] Run focused unit tests plus real PostgreSQL round-trip/constraint tests,
  including concurrent dev creation and reconstruction of the store/service.
  Existing account tests supply regression coverage; do not duplicate their suite.

## Task 2 — Wire identity and connection ownership

**Files:** `internal/config/config.go` and tests, `cmd/main.go`,
`internal/websocket/server.go`, relevant startup/stack tests, `.env.example`.

- [ ] Test dev/prod/default/invalid environment configuration and require the
  existing `DATABASE_URL` for the server in both modes.
- [ ] Share only `config.LoadEnvironment() (Environment, error)` with the client;
  the client must not need database credentials. Server config loads database
  settings separately. Create/ping the pool with a bounded startup context and
  wire both profile and game repositories in both environments.
- [ ] Make server construction explicitly take the guest service and development
  flag: `NewServer(address string, games gameplay.Service, guests *guest.Service,
  development bool) *Server`. Isolated transport tests may use a fake store;
  actual development startup must use PostgreSQL.
- [ ] Add `POST /guests` in production: status 201, ProfileID/token JSON and
  `Cache-Control: no-store`. Authenticate production WebSockets using an
  Authorization bearer header; reject public UUID overrides and invalid/missing
  credentials before upgrading. Storage errors return 503, never a bypass.
- [ ] In dev, generate/parse the UUID and ensure its database row before upgrading.
  Disable production bootstrap in dev. Do not place tokens in URLs or logs.
- [ ] Atomically reserve the resolved profile before WebSocket upgrade. Keep a
  small Server-owned reservation map separate from game membership; release only
  the reservation owned by that request, after session and disconnect cleanup.
  Failed upgrades release their reservation; rejected duplicates never release
  or cancel the original connection's game or matchmaking ticket.
- [ ] Hold connection ownership through idle time, matchmaking and game completion.
  Completing a game releases its seat, not its live connection reservation.
  Stale connections retain ownership until close/error/heartbeat cleanup;
  there is no silent takeover. Do not hold the reservation lock over I/O.
- [ ] Replace obsolete duplicate-idle acceptance tests. Cover concurrent admission
  with one winner, duplicate attempts during queue/play, original-session survival,
  ownership after game completion, failed upgrade, and successful reconnect after
  observed cleanup. Reuse existing lifecycle tests rather than duplicating them.

## Task 3 — Persist the production client's guest credential

**Files:** `cmd/chessli/main.go`, `internal/tui/app.go`, `connection.go`,
affected callers/tests and `Makefile`; new `internal/guestclient/identity.go`
and focused tests.

- [ ] Test load-or-bootstrap against temporary files and an HTTP test server.
  Subsequent launches reuse the same credential without another profile creation.
- [ ] Save only ProfileID/token at `filepath.Join(os.UserConfigDir(), "chessli",
  "identity.json")`, using owner-only permissions where supported and writing
  without clobbering an existing identity. Concurrent first launches must not
  overwrite one another; using the winner or failing clearly is sufficient.
- [ ] Treat a missing file as first use. Corrupt/unreadable files, invalid saved
  tokens, connection errors and failed saves stop startup clearly instead of
  silently switching identity. Save before entering gameplay.
- [ ] Dev skips disk/bootstrap entirely and keeps `-profile-id`; prod rejects
  that override. Test this distinction directly. Do not add named profiles.
- [ ] Extend TUI connection setup to carry the bearer token without URL exposure;
  propose `tui.New(source, address, token)`, with empty token in dev. Preserve
  connection cancellation/deadlines and existing startup active-game resume.
- [ ] A second production client reuses the saved guest and receives HTTP 409.
  Display that conflict clearly; leave the first client's game untouched.
- [ ] Run focused tests and cross-compile the client for Windows/macOS in a
  temporary directory. Cross-compilation is not a claim of runtime testing there.

## Task 4 — Save complete games without blocking play

**Files:** `internal/gameplay/service.go`, `game_record.go`, focused service
tests; `cmd/main.go`; a small mapping helper in `internal/persistence/game/`
with repository/integration tests as needed. Keep all GameService methods in
service.go and storage representations inside persistence packages.

**Interface:** export the existing detached completion record/move value types
now that they have an actual application caller. Add implementation-only
`GameService.SetGameFinishedHandler(func(CompletedGameRecord))`; keep
`gameplay.Service` unchanged. The persistence adapter maps that record to its
existing `CompletedGame`/`Move` storage types and calls the existing repository.

- [ ] Write focused completion-observation tests for move/engine endings,
  resignation, accepted draw, timer timeout and lazy expiration from reads or
  commands, including commands that discover timeout but return an error.
- [ ] Reuse the existing terminal-observation locations: MakeMove, Resign,
  AcceptDraw, ResumeGame, GameState, OfferDraw, DeclineDraw and expireFromTimer.
  Under gameplay.mu, capture a detached completed record and mark it observed
  once. Repeated reads/resume and timer races must not create repeated saves.
- [ ] Run the completion callback through the existing notification WaitGroup,
  independently from timeout publication. Perform database work outside the
  gameplay and transport dispatch mutexes. Do not put saving before final-state
  delivery in the same callback; a slow/failed database must not block results.
- [ ] Return persistence errors normally and log them once at the application
  callback boundary with the game ID. Preserve the terminal result. Follow
  existing context conventions; add no special save-timeout policy.
- [ ] Reuse the existing Close join for started callbacks; close the database pool
  afterward. Add no worker pool, persistence queue or retry/lifecycle framework.
- [ ] Use channel-controlled saver tests to show final publication and unrelated
  play continue while saving blocks; verify failure behavior and shutdown join.
  Add no sleeps or redundant copies of existing chess-outcome tests.

## Task 5 — Verify the feature and update affected documentation

**Files:** `README.md`, `docs/manual-error-testing.md`,
`docs/connection-lifecycle.md`, `docs/roadmap.md`, an ADR for the identity and
completion-save boundary, and this ignored feature plan's verification notes.

- [ ] Exercise real PostgreSQL in dev and prod: persisted participants, complete
  game record including original time control/moves/clocks/completion metadata,
  and reload through a fresh repository/application instance. Dev explicit UUID
  reconnect reuses the row. Use tests with temporary credential files for prod;
  interactive two-profile prod testing is not required.
- [ ] Manually run four dev clients for independent profiles; finish a game,
  restart the server and verify its stored record directly in the database.
  Also verify same-profile conflict and reconnect after the first client closes.
  This validates persistence without adding a guest history API or UI.
- [ ] Document both environments, required database/migrations, dev UUID bypass,
  production saved credentials and same-profile ownership. Update affected
  protocol examples and obsolete behavior tests.
- [ ] Run `go test ./...`, `go vet ./...`, `go test -race ./...`, relevant real
  PostgreSQL integration tests and `git diff --check`. Database cases must run,
  not merely skip. Broaden/repeat checks only for changes or unresolved concerns.
- [ ] Perform one read-only final review and resolve substantive findings. Report
  verification and limits. Keep all implementation work uncommitted unless asked.

## Self-review and handoff boundary

The environment switch no longer disables persistence. Dev participants exist
before games can reference them; both modes use the same completion-save path.
Connection ownership is distinct from game membership. The public gameplay
contract stays storage-independent. Server-only database config is not loaded by
the client. Registration-only features are not implemented or exposed.

The environment default, duplicate-connection rejection and expiry deferral above
were assistant proposals, not separately confirmed user choices. Preserve that
provenance; the user's latest request moves this feature plan to an implementation
handoff. Use these defaults unless the user changes them or a material conflict
is discovered. Save errors use ordinary return/log handling as explicitly directed
by the user; reliability policy is not an open decision here. The implementation
scope is this iteration of durable-history, not a new initiative or the rest of
the roadmap.
