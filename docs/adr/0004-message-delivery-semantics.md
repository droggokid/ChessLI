# ADR 0004: Outgoing Message Delivery Semantics

Status: Accepted

## Scope

Applies to `Session.Send`, the session writer loop, and code that broadcasts or
otherwise sends `protocol.ServerEnvelope` values.

## Context

Sessions queue outgoing messages asynchronously. Shutdown, caller cancellation,
queue saturation, and network failure can race with producers attempting to
send. Imposing strict ordering between these events would add synchronization
without proving that the peer received a message.

## Decision

A successful `Session.Send` means that the session accepted the message into
its outgoing queue. It does not mean that the writer wrote the message to the
network or that the peer received it.

`Send` is a non-blocking queue operation. Network writing belongs to the
session's writer loop. Confirmed delivery requires a separate protocol-level
acknowledgement.

The WebSocket handler serializes commands, admission, disconnect publication,
and expiration publication through outgoing queue acceptance. This prevents
stale active snapshots following a terminal state and registration after
terminal membership cleanup. Gameplay expiration callbacks are asynchronous
so they cannot re-enter a transport operation holding that dispatch lock.

The current implementation uses one dispatch mutex for the in-memory service.
This deliberately limits parallelism across games; replace it with per-game
dispatch if throughput requires it. Do not put database or network I/O inside
this critical section.

A source send failure is returned after peer delivery is attempted. Peer
enqueues for a committed action are independent of the source request's
cancellation, always clear its request ID, and remain finite and non-blocking.
Matchmaking admission publishes presence to already registered peers.

### Constraints

- `Send` returns `protocol.ErrSessionNotRunning` when it observes that the
  session is not running.
- After observing a running session, `Send` returns the caller context error
  when the context is already canceled or expired when checked.
- `Send` returns `protocol.ErrSessionQueueFull` rather than blocking when the
  outgoing queue has no capacity.
- A successful return reports queue acceptance only.
- The outgoing channel is not closed during normal shutdown because concurrent
  senders do not own it; cancellation and run state signal shutdown.
- Do not add synchronization intended to turn queue acceptance into network or
  peer acknowledgement.
- Stronger delivery guarantees require an explicit acknowledgement mechanism
  and a separate architectural decision.

### Invariants

- Queue acceptance, network write success, and peer receipt remain separate
  concepts.
- Callers do not infer remote delivery from a successful `Send`.
- Slow clients do not cause `Send` to block its caller.
- A message accepted immediately before or concurrently with shutdown may not
  reach the peer.

## Alternatives Considered

### Block producers until queue capacity is available

Rejected because:

- A slow client would apply unbounded latency to application work.
- It still would not prove network or peer delivery.

Rejected despite:

- It can reduce queue-full errors during short bursts.

### Acknowledge each network write from the writer loop

Rejected because:

- A successful socket write still does not prove that the peer processed the
  message.
- Per-message coordination would materially complicate the session API.

Rejected despite:

- It would provide a stronger local guarantee than queue acceptance.

## Consequences

Positive:

- Senders have bounded, explicit behavior under backpressure.
- Session shutdown does not require closing a channel used by concurrent
  producers.
- The API's guarantee matches what it can actually prove.

Negative:

- Messages can be dropped because of queue saturation or shutdown.
- Applications needing confirmed processing must add protocol-level
  acknowledgements.
