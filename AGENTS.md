# Repository Guidelines

## Project Structure & Module Organization

ChessLI is a Go module for chess domain modeling. The entry point lives in `cmd/main.go`. Core code is under `internal/chess`: `board` owns board setup and square access, `board/models` contains shared domain types such as `Position`, `Color`, `Spot`, and `Piece`, `board/pieces` contains concrete piece implementations and move helpers, and `game` owns players, turn state, captures, and move orchestration. There are currently no dedicated test directories or assets.

## Build, Test, and Development Commands

Use the Makefile targets from the repository root:

```sh
make run    # run ./cmd
make build  # compile all packages
make test   # run all Go tests
make fmt    # format Go code with go fmt
make vet    # run go vet
make tidy   # update go.mod/go.sum
```

Direct Go commands are also acceptable, for example `go test ./...` or `go run ./cmd`.

## Coding Style & Naming Conventions

Use standard Go formatting via `gofmt` or `make fmt`; tabs are handled by the formatter. Keep package names short and lowercase. Constructors use `NewType`, such as `NewBoard`, `NewPlayer`, and `NewBasePiece`. Keep model types in `internal/chess/board/models` free of concrete package dependencies to avoid import cycles. Prefer board/game orchestration for state changes, while pieces should describe movement behavior. Structs should have corresponding interfaces where practical so gomock-based tests can be added later.

## Testing Guidelines

Use Go’s built-in `testing` package and keep tests next to the code they cover with `_test.go` suffixes. Name tests by behavior, such as `TestMoveRejectsOwnPieceCapture`. Prefer table-driven tests with `t.Run` when checking variants of the same behavior. Keep one-off tests direct when a table would add noise.

Test helpers must live in `_test.go` files, usually `test_helpers_test.go`, and must call `t.Helper()` when they receive `*testing.T`. Avoid panics in tests; fail through `t.Fatalf` or `t.Fatal`. Prefer comparing domain values directly and keep fixtures small enough that the expected behavior is visible in the test.

For gomock, keep `//go:generate go run go.uber.org/mock/mockgen@...` directives near the interface they generate from. Generate mocks into `_test.go` files when they are only used by tests. Run `make generate` after interface changes, then `make test` and `make vet`.

## Commit & Pull Request Guidelines

Recent commits use short, imperative summaries such as `clean up refactoring` and `game models and board`. Keep commit messages concise and focused on one logical change. Pull requests should include a brief description, the commands run (`make test`, `make vet`, etc.), and notes about known limitations or incomplete chess rules. Include screenshots only when UI work is added.

## Agent-Specific Instructions

Agents are not allowed to make direct file edits without explicit user permission. When reviewing or advising, inspect the relevant files first and ground feedback in the current code. Preserve user changes in the working tree and avoid broad refactors unless requested.
