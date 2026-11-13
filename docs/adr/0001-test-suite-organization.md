# ADR-0001: Organize tests into unit, integration, and stack suites

## Status

Accepted

## Date

2026-11-13

## Context

Tests were spread across files named after features, lifecycle phases, and
logging concerns. CI ran the full suite twice, while the dedicated stack command
selected WebSocket tests by name and omitted every TUI stack test.

We need useful regression coverage, predictable filenames, and independent CI
results for each test level. Go's package visibility also matters: moving a test
to another directory prevents it from accessing package-private code.

## Decision

| Suite | Scope | Location and naming |
| --- | --- | --- |
| Unit | One component's rules and behavior, with controlled dependencies | Beside its source file, using the same basename: `service.go` and `service_test.go` |
| Integration | Interactions between components or with real transports and PostgreSQL | Public-boundary tests in `tests/integration/`; tests needing private access stay beside the component in `*_integration_test.go` |
| Stack | Player flows through the assembled server and clients over local sockets | All tests in `tests/stack/` |

Start with the smallest test that can catch the relevant failure. Add an
integration test when correctness depends on a real component boundary, such as
SQL constraints, transport cancellation, or serialization across a socket, and
a unit test cannot provide that evidence. Add a stack test for a critical player
flow whose failure depends on the assembled components working together.

A feature does not automatically need tests at every level. Keep branching and
edge cases in unit tests where possible. Broader tests must cover a distinct
integration risk rather than repeat assertions already protected at a lower
level. Prefer extending an existing scenario when it covers that risk clearly.

Every integration test function starts with `TestIntegration`. Native `go test`
name filters select or exclude these tests, including colocated integrations.
Stack tests are selected by directory, so adding a TUI test cannot silently
exclude it from the stack command. Build tags and exported test hooks are not
needed.

Stack tests use the real WebSocket server, gameplay service, and TUI, with
repository doubles. PostgreSQL round trips, constraints, and migrations belong
to the integration suite. Stack tests therefore do not verify deployment or
the database wiring of the executable.

Keep tests that protect observable behavior, validation boundaries, lifecycle
ordering, and concurrency invariants. Consolidate overlapping scenarios while
preserving distinct failure cases. Remove checks that merely repeat constructor
assignments or test a dependency's behavior. Use controlled clocks or observable
events for concurrency tests.

Run each suite independently:

```sh
make test-unit
CHESSLI_TEST_DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable' make test-integration
make test-stack
```

Each target enables the race detector. CI runs three independent jobs and starts
PostgreSQL only for integration tests. The integration target requires
`CHESSLI_TEST_DATABASE_URL` so missing configuration cannot silently skip database
coverage. Database tests create isolated schemas and remove them at cleanup.

`make test` and `make test-race` remain available for all suites. In these broad
runs, PostgreSQL tests skip when the test database URL is absent.

## Alternatives considered

Moving every integration test into one directory would require exposing private
details or rewriting focused component regressions as larger flows. Keeping
private integrations colocated preserves that coverage without new APIs for test
hooks.

Build tags could separate suites, but would complicate shared test fixtures and
prevent ordinary `go test ./...` from compiling all tests together. Name filters
and directory selection are sufficient here.

## Consequences

New integration tests must follow the function prefix as well as the file
convention. Suite selection controls execution; Go still compiles all test files
in each selected package. Unit filenames remain predictable, while integrations
can cover multiple source files without an artificial one-to-one mapping.
