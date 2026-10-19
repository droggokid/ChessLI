# Application code organization

Implement the approved organization proposal without changing application behavior.
Keep the public gameplay port, package dependencies, protocol, logging, retention,
and shutdown contracts. Keep handwritten concrete types' methods beside their definitions.
Helper files contain only receiver-free functions with explicit inputs.

## Steps

1. Extract the matchmaking queue; keep queue mutations and game registration under
   the existing service mutex, and cancellation ownership on GameService.
2. Split game runtime synchronization/timers from synchronous gameState rules.
   Extract draw-offer bookkeeping and engine conversions. All state access remains
   under the game mutex; snapshots remain detached copies.
3. Extract transport admission and stateless decoding/mapping. Handler retains
   dispatch ordering and matchmaking waiters; gameSessions retains membership and
   publication. Admission has no lock, goroutine, or reference back to Handler.
4. Align fixtures and ownership documentation, inspect method locality, then run
   the full tests, race detector, formatting check, and vet.

## Review boundaries

- Leave changes uncommitted in the current development checkout for review.
- Use existing behavior and concurrency tests for this refactor; add a regression
  test only if an uncovered behavior or defect is discovered.
- Do not wire repositories into application code, add interfaces, change error
  messages, or change cancellation/notification ordering.
- Preserve timer generation/version/deadline guards, peer publication after source
  cancellation, reservations, and terminal membership cleanup.

## Progress

- Baseline: `go test ./...` passed before edits.
- Queue extraction: existing gameplay tests passed.
- Game runtime/state split: existing gameplay tests passed; fixtures now access
  state explicitly rather than fields directly on the runtime object.
- Admission, mapping, and decoding extraction: gameplay and WebSocket tests passed.
- Matchmaking entry remains on Handler because it schedules the waiter and owns
  its reservation lifetime; completed matches use the admission component.
- Existing behavior tests are the regression gate; no new feature tests or
  database integration were added.
- Final verification: `go test ./...`, `go test -race ./...`, and `go vet ./...`
  passed. Formatting and `git diff --check` passed.
- Method-locality inspection covered 13 handwritten receiver types; none has
  methods spanning files. Generated code is excluded from this convention.
- Independent read-only review found no actionable behavior regressions.
- Changes remain uncommitted for user review.
