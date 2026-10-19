# Repository Guide

## Project and commands

ChessLI is a Go chess server with a terminal client. `cmd/main.go` wires the
server; `cmd/chessli/` runs the client. Under `internal/`, `gameplay` owns
live games, clocks, and matchmaking; `game`, `profile`, and `identity` hold
domain models; `websocket` serves connections and `websocket/protocol` defines
the wire contract. `tui` contains the client, `persistence` the storage
adapters, and `database` the SQL, migrations, and generated queries.
Tests live beside the code; runnable examples are in `demo/`.

Run commands from the repository root:

```sh
make run         # run the server
make run-tui     # run the terminal client; optional PROFILE_ID=uuid
make build       # build the server (build-tui for the client)
make test        # go test ./...
make test-race   # go test -race ./...
make fmt         # format Go code
make vet         # go vet ./...
make check       # formatting check, vet, and tests
make generate    # regenerate sqlc code and Go-generated files
make tidy        # update module metadata when dependencies change
```

See the Makefile for database and other development targets. Direct Go commands
are also fine.

## Working approach

Read the relevant code before proposing or implementing changes. Prefer the
smallest coherent solution, idiomatic Go, and existing conventions where they
help. Use interfaces and abstractions for demonstrated needs. Prefer the
standard library and existing dependencies when suitable; weigh new dependencies
against their maintenance cost and keep their types internal unless exposing
them is intentional.

Exercise engineering judgment: explain meaningful trade-offs and challenge
existing choices when a better solution is justified. Consult the relevant
[ADRs](docs/adr/) for architectural context; revise or supersede decisions when
their rationale no longer holds. Record significant architectural decisions,
rather than ordinary implementation or style choices.

Implement requested changes within their scope, preserving unrelated user
changes. Update affected callers, generated code, tests, and documentation as
needed. For protocol changes, check the examples in `README.md` and
`docs/manual-error-testing.md` and update the affected material.

## Go conventions

- Prefer concrete types over premature interfaces.
- `gameplay.Service` is the transport-independent gameplay contract; keep
  implementation-only lifecycle/configuration methods off it.
- Keep all `GameService` methods in `internal/gameplay/service.go`.
  `service_helpers.go` is only for receiver-free supporting code.
- In handwritten production code, keep each concrete type and all its methods
  in one file. Extract a cohesive responsibility when that file grows; do not
  scatter a receiver's methods. Generated code follows its generator's layout.
- Helper files contain receiver-free functions with explicit inputs, not methods
  disguised as functions taking the entire owning object.
- Avoid mutable package-level state.
- Prefer the standard library and existing dependencies.
- Do not expose third-party types from public APIs unless intentional.

## Verification and communication

Use Go's testing package for meaningful behavior tests with clear fixtures.
Synchronize concurrency tests through observable events or controlled clocks
rather than arbitrary sleeps. For Go changes, run `go test ./...` and
`go vet ./...`; include `go test -race ./...` for concurrency changes.
Choose proportionate checks for documentation or configuration changes.

Be concise, candid, and concrete. Ask when missing information materially
affects the result; give brief updates during longer work. Summarize what
changed, the checks run, and any remaining limitations in handoffs and PRs.
