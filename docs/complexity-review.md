# Handler, callback, and channel complexity review

Reviewed on 2026-11-13 through read-only research across gameplay, WebSockets,
the TUI, persistence, executable wiring, and supporting tests.

The six findings below are agreed items for step-by-step review. Completed items
record their implementation and verification below; unchecked items remain open
for review. The matchmaking alternative needs a design decision before work
begins; agreement with the finding does not settle its API or cancellation rules.

## Progress

- [x] 1. Remove the draw-response operation parameter.
- [x] 2. Return request data directly from the Play screen.
- [x] 3. Handle Dashboard navigation directly in the parent.
- [x] 4. Review matchmaking outcomes and cancellation ownership.
- [ ] 5. Review the session handler callback's value.
- [ ] 6. Consolidate completion bookkeeping.

Keep this order when reviewing the findings. Record the decision and verification
for each item as it is resolved; an explicit decision to keep a mechanism also
resolves its review item.

## 1. Remove the draw-response operation parameter

Source: [`Handler.Handle` and `handleDrawResponse`](../internal/websocket/handler.go).

Previously, `handleDrawResponse` received a function solely to select `AcceptDraw`
or `DeclineDraw`. The selection was already represented by the envelope's message
type, but readers had to follow the caller to discover which operation ran.

Implemented on 2026-11-13: combined the accept/decline dispatch cases and branched
on `message.Type` inside `handleDrawResponse`, calling the service methods
directly. Shared decoding, application-error mapping, and state publication stay
in the helper. The small increase in lines makes the actual operation visible.

Verification: `go test ./...` and `go vet ./...` passed. Existing
[`handler_test.go`](../internal/websocket/handler_test.go) cases cover command
fields, dispatch to both service methods, publication to both players, request
IDs, and invalid-message handling. No protocol or concurrency behavior changed.

## 2. Return request data directly from the Play screen

Sources: [`play.Model.request` and `event`](../internal/tui/screens/play/model.go),
[`tui.Model.Update`](../internal/tui/app.go).

The Play screen wraps an already-created `RequestMsg` in a command. Bubble Tea
executes that command, returns the message to the parent, and the parent starts
the actual network operation. This adds an asynchronous round trip for data
already available during the update.

Proposed change: let the child return an optional request value and let the
parent schedule the send and response deadline immediately. The child's update
method already has a custom signature, so it need not mirror `tea.Model`.

Preserve request IDs, pending-request checks, connection readiness, response
deadlines, acknowledgements, and unknown-outcome handling. Startup auto-resume
also produces a request through `event` and must use the same path. Keep actual
network operations asynchronous.

## 3. Handle Dashboard navigation directly in the parent

Sources: [`dashboard.Model.Update`](../internal/tui/screens/dashboard/model.go),
[`tui.Model.Update`](../internal/tui/app.go).

Dashboard Enter creates a command whose only result is `StartGameMsg`. The parent
receives it and changes the active screen, although it already owns navigation
and key routing.

Proposed change: handle Enter while Dashboard is active in the parent's existing
key handling. Remove `StartGameMsg`, its command closure, and its receiving case.
Preserve Enter behavior while the dashboard is loading or showing an error and
leave Enter on other screens to their existing handlers.

## 4. Review matchmaking outcomes and cancellation ownership

Sources: [`GameService.EnterMatchmaking`](../internal/gameplay/service.go),
[`matchQueue` and `waitingPlayer`](../internal/gameplay/match_queue.go),
[`MatchTicket`](../internal/gameplay/results.go),
[`Handler.handleMatchmaking` and `awaitMatch`](../internal/websocket/handler.go).

A queued player's lifetime spans a result channel, context cancellation channel,
cleanup callback, callback stop function, opportunistic cancellation scan,
transport waiter goroutine, and wait group. Ownership is spread across service
and transport code.

Design candidate: return either a queued outcome or both matched assignments
from nonblocking admission. The handler keeps pending session/request data and
publishes both assignments when the second player arrives. Disconnect explicitly
cancels an unmatched entry. This could remove tickets, assignment channels,
waiter goroutines, and cleanup callbacks.

This changes the current guarantee that canceling the admission context removes
an unmatched entry even when nobody consumes its ticket. Define cancellation
ownership and queue-entry identity before changing the API. Preserve immediate
re-admission after cancellation, cancellation eligibility before pairing,
acknowledgement-before-assignment ordering, peer publication, recovery after a
committed match, and shutdown cleanup. Keep the gameplay contract independent
of WebSocket sessions. Record a significant architectural decision in an ADR.

A blocking matchmaking call inside the current handler is unsuitable: the
handler holds the dispatch mutex, preventing the opponent from entering, and
the call would stall the session reader. Moving it into a goroutine alone does
not remove the asynchronous design.

## 5. Review the session handler callback's value

Sources: [`HandlerFunc`, `Session.Run`, and `readLoop`](../internal/websocket/session.go),
[`Server.handleConnection`](../internal/websocket/server.go).

`HandlerFunc` is threaded through the session lifecycle, with `Handler.Handle`
as its only production target. Passing a concrete `*Handler` could make calls
easier to navigate and remove the custom function type.

This is optional: the callback is also a small, useful seam for isolated
transport tests, including readiness and heartbeat behavior. Compare that value
with the added coupling before replacing it. Keeping the callback is a valid
outcome if the concrete version makes tests or ownership harder to understand.

## 6. Consolidate completion bookkeeping

Sources: [`GameService.notifyFinished`](../internal/gameplay/service.go),
[`game`](../internal/gameplay/game.go).

`completionScheduled` and `expirationNotified` separately guard persistence and
timeout publication, although both actions originate in the same mutex-protected
completion path and the terminal outcome does not change.

Proposed change: use one completion guard, then schedule persistence and optional
timeout publication together. Remove the redundant flag and overlapping checks.
Preserve timer shutdown, one-time side effects, closed-service behavior, and the
ability for either a command or a timer to detect completion first.

## Mechanisms with demonstrated purposes

- The outgoing WebSocket channel preserves queue order and prevents slow socket
  writes from blocking gameplay dispatch. A successful send means queued, not
  delivered.
- Independent read, write, and heartbeat operations let network activities
  progress concurrently. Their error channel also joins the loops on shutdown.
- Bounded shutdown waits require coordination with the shutdown context.
- Timeout publication must remain asynchronous with the current locks: command
  handlers hold the transport dispatch mutex while calling gameplay, and timeout
  publication acquires that same dispatch mutex. Synchronous publication from a
  gameplay operation can deadlock, even if the gameplay mutex is released first.
- Background persistence keeps slow writes and retries outside the gameplay
  mutex, while service shutdown waits for outstanding saves.
- Clock and color functions provide small deterministic test seams.
- Actual Bubble Tea network commands perform asynchronous work. Connection
  cancellation and operation tracking remain useful because the framework does
  not wait for every outstanding command during shutdown.
- Test callbacks and channels generally coordinate observable events and force
  meaningful races. Their presence alone is not a reason to remove them.

Replacing a function with a one-method interface does not by itself simplify
control flow. Judge each change by whether it makes behavior and ownership easier
to follow while preserving these requirements.

## Verification

The audit ran no builds or tests and changed no production code. For each Go
implementation, obtain command approval and run `go test ./...` and
`go vet ./...`; include `go test -race ./...` for concurrency changes. Use
existing behavior tests where sufficient and add coverage only for meaningful
gaps introduced or exposed by the change.
