# Basic Service Completion Plan

Status: Historical. The completed lifecycle work remains documented here;
persistence moved to [Persistence Foundation Plan](persistence-foundation.md).

ChessLI already finishes games through checkmate, stalemate, timeout,
resignation, draw agreement, and engine-detected automatic draws. The remaining
work is connection and terminal-game lifecycle.

## 1. Define player identity and reconnect policy

Profiles are generated per WebSocket connection, so a disconnected player
cannot resume their seat. Decide whether the basic service is intentionally
anonymous and non-reconnectable, or provide a resumable authenticated player
credential. Reconnect support depends on this decision.

## 2. Add transport and process health

- Use native WebSocket ping/pong via `coder/websocket`, rather than JSON ping
  messages.
- Let `Session` own the heartbeat goroutine and stop it with the session; a
  missed pong closes the session.
- Add a separate HTTP `/healthz` endpoint for load balancers and process health.

## 3. Publish connection changes

- When a session disconnects, broadcast an updated `game.state` so the
  opponent sees `connected: false`.
- Do the same after a successful reconnect, if reconnect is implemented.
- Keep clocks running during a disconnection as the smallest normal policy, or
  explicitly choose an abandonment timeout or auto-resign policy.

## 4. Complete terminal-game lifecycle

- Broadcast the final authoritative state once.
- Release both sessions from `GameSessions` so they can create or match into a
  new game without reopening their WebSocket connection.
- Persist completed games for player history and analysis; do not add an
  arbitrary in-memory retention window.
- Treat replay and active-game recovery as separate features.

## 5. Close reliability gaps

- Do not start a match for a session that disconnected while matchmaking was
  completing.
- Add structured logs or metrics for connections, heartbeat timeouts,
  queue-full drops, matches, and terminal-game reasons.
- Keep the existing inbound-size and outbound-queue limits. Add origin checks,
  authentication, and rate limiting when the service becomes browser-facing or
  public.

## 6. Verify end-to-end behavior

Add WebSocket stack tests for:

- removal of a dead peer after heartbeat timeout;
- disconnect and reconnect state broadcasts;
- one final state broadcast, session reuse, and finished-game expiry; and
- disconnection during matchmaking without a stranded game.

## Deliberate deferrals

Do not add persistence, rematches, spectators, draw claims (threefold or
fifty-move), ratings, or full authentication until the reconnectable session
lifecycle is complete.
