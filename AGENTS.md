# Repository Guidelines

## Project Structure & Module Organization

ChessLI is a Go WebSocket server for real-time chess games. The entry point and
dependency wiring live in `cmd/main.go`. Core code is under `internal/`:
`gameplay` owns games, clocks, matchmaking, commands, snapshots, and application
orchestration; `identity` owns stable game and profile identifiers; `websocket`
owns the server, sessions, handlers, and mappings; `websocket/protocol` owns the
wire contract; `config` and `log` provide process-level support. Runnable
examples live under `demo/`. Tests are kept next to the code they cover.

## Build, Test, and Development Commands

Use the Makefile targets from the repository root:

```sh
make run        # run ./cmd
make build      # compile the server
make test       # run all Go tests
make test-race  # run all tests with the race detector
make fmt        # format Go code with go fmt
make fmt-check  # verify Go formatting
make vet        # run go vet
make check      # run formatting, vet, and tests
make generate   # run go generate directives
make tidy       # update go.mod/go.sum
```

Direct Go commands are also acceptable, for example `go test ./...` or `go run ./cmd`.

## Conversational Style

- Be an insightful, candid, collaborative thought partner with a clear point of
  view.
- Lead with the outcome, then explain only the reasoning and trade-offs that
  matter.
- Match my technical level and tone; use plain language without becoming
  simplistic.
- Stay concise without becoming curt. Use minimal formatting.
- Ask questions only when missing information materially changes the result or
  creates risk.
- When disagreeing, be direct, concrete, and constructive.
- During tool-heavy work, give brief updates and make the final answer self-
  contained.

## Go Conventions

Follow idiomatic Go and the Uber Go Style Guide.

Repository-specific rules:

- Prefer concrete types over premature interfaces.
- `gameplay.Service` is the transport-independent gameplay contract; keep
  implementation-only lifecycle/configuration methods off it.
- Keep all `GameService` methods in `internal/gameplay/service.go`.
  `service_helpers.go` is only for receiver-free supporting code.
- Avoid mutable package-level state.
- Prefer the standard library and existing dependencies.
- Do not expose third-party types from public APIs unless intentional.


## Testing Guidelines

Use Go’s built-in `testing` package and keep tests next to the code they cover with `_test.go` suffixes. Name tests by behavior, such as `TestMoveRejectsOwnPieceCapture`. Prefer table-driven tests with `t.Run` when checking variants of the same behavior. Keep one-off tests direct when a table would add noise.

Test helpers must live in `_test.go` files, usually `test_helpers_test.go`, and must call `t.Helper()` when they receive `*testing.T`. Use explicit test helpers for shared fixture setup instead of hiding important setup in broad abstractions. Avoid panics in tests; fail through `t.Fatalf` or `t.Fatal`. Prefer comparing domain values directly and keep fixtures small enough that the expected behavior is visible in the test. Do not use arbitrary sleeps to synchronize concurrency tests; wait on an observable event or use a controlled clock.

For gomock, keep `//go:generate go run go.uber.org/mock/mockgen@...` directives near the interface they generate from. Generate mocks as normal package files named after the interface source, such as `game_service_mock.go`. Run `make generate` after interface changes, then `make test` and `make vet`.

Run `go test ./...` for normal changes and `go test -race ./...` for
concurrency-related changes. Run `go vet ./...` before handing off production Go
changes.

## Commit & Pull Request Guidelines

Recent commits use short, imperative summaries such as `clean up refactoring` and `game models and board`. Keep commit messages concise and focused on one logical change. Pull requests should include a brief description, the commands run (`make test`, `make vet`, etc.), and notes about known limitations or incomplete chess rules. Include screenshots only when UI work is added.

## Documentation Guidelines

Keep the manual WebSocket flow in `README.md` and the payload examples in `docs/manual-error-testing.md` synchronized with the current protocol. Whenever message types, payload schemas, validation rules, error codes, error messages, or observable flow behavior change, update both documents in the same change. Keep JSON examples ready to paste into `websocat`, and clearly identify application errors that are not currently reachable through the WebSocket API.

## Architecture Decisions

Accepted architecture decisions live in `docs/adr/`.

Before changing package boundaries, concurrency semantics, lifecycle ownership,
protocol behavior, public APIs, interface ownership, context propagation, or
dependency direction, read the applicable ADRs.

Treat ADR constraints and invariants as requirements. Do not silently work
around or contradict an accepted ADR. If a requested change conflicts with an
ADR, point out the conflict and update or supersede the ADR as part of the
change when appropriate.

Do not create ADRs for ordinary implementation details or style choices. Create
one when a decision is cross-cutting, surprising, difficult to reverse, or
represents an intentional trade-off.

## Agent-Specific Instructions

Agents are not allowed to make direct production-code edits without explicit user permission. Agents have standing permission to update `README.md`, files under `docs/`, and test files when needed to keep documentation and verification aligned with the current behavior. When reviewing or advising, inspect the relevant files first and ground feedback in the current code. Preserve user changes in the working tree and avoid broad refactors unless requested. Prefer the smallest coherent change that solves the requested problem.
