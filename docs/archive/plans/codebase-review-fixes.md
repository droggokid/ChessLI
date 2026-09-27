# Codebase Review Fixes

Status: Completed for the scoped review fixes (2026-09-27). Archived.

## Implementation

- Preserve peer delivery when the acting session fails or its context is canceled.
- Serialize transport commands, admission, disconnect notifications, and timeout publication through queue acceptance. Use one dispatch mutex for the current in-memory service; split by game if throughput requires it.
- Run expiration notifications asynchronously, owned and joined by GameService, so callbacks cannot re-enter transport dispatch.
- Keep finished-game admissions read-only; publish matchmaking presence changes to existing peers.
- Return admission failures independently of error-message delivery.
- Update renamed repository tests and regenerate mocks.
- Remove duplicated broadcasting, compatibility aliases, the clamp helper, and unused ignore-template rules.
- Synchronize protocol documentation and record the delivery/lifecycle guarantees in the existing ADRs.

## Verification

- Failed/full/canceled source delivery still attempts healthy peers, with peer request IDs cleared.
- Concurrent terminal transitions and admissions leave sessions reusable and never publish active state after a terminal state.
- Matchmaking updates both players' presence; admission failure sends only an error.
- Expiration callbacks do not block command completion and are joined at shutdown.
- Run generation, normal/race tests, vet, formatting, SQL compilation, and generated SQL comparison.

Verified: `make generate`, `make check`, `make test-race`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `sqlc compile`, `sqlc diff`, and `git diff --check` all passed. Regression coverage includes real WebSocket matchmaking presence and automatic timeout delivery through the gameplay-to-server callback.

## Separate persistence work

Persistence wiring, original time-control/per-ply clock capture, transactional game-and-move saving, and durable-history-backed retirement remain in the persistence foundation work. Do not delete retained games or invent a retention window in this correctness pass.
