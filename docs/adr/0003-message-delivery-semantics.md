# ADR 0003: Outgoing Message Delivery

Status: Accepted.

## Context and decision

Outgoing messages are queued asynchronously. Successful `Session.Send` means
queue acceptance, not a network write, peer receipt, or peer processing.

Enqueueing is non-blocking so a slow client cannot stall application work.
`Send` returns `ErrSessionNotRunning` if it observes a non-running session;
otherwise it checks caller cancellation and returns the context error if
already canceled. A full queue returns `ErrSessionQueueFull`. Queue acceptance
racing with shutdown may succeed even though the message never reaches the
peer. These are bounded local guarantees, not confirmed delivery.

State changes, admission, disconnect publication, and expiration publication
are coordinated through queue acceptance. An older active snapshot must not
follow a terminal snapshot, and admission must not re-register membership
after terminal cleanup. Matchmaking admission publishes presence to already
registered peers.

After a committed action, peer enqueue attempts proceed independently of the
source send's failure or request cancellation; the source error is returned
after those attempts. Peer failures are logged. These attempts remain finite
and non-blocking. Request correlation follows
[ADR 0004](0004-protocol-compatibility.md).

## Rationale and trade-offs

Blocking for queue space adds latency without proving receipt. A socket-write
acknowledgement still cannot prove peer processing; confirmed delivery would
need a protocol-level acknowledgement and an explicit compatibility decision.

The current handler uses one dispatch mutex, and expiration callbacks run
asynchronously to avoid re-entering a transport operation waiting on them.
That lock limits parallelism across games. Its scope and replacement mechanism
are implementation choices; the ordering guarantees above are the contract.
Database and network I/O stay outside the serialized state/publication work so
slow external operations do not block unrelated gameplay.

Backpressure, disconnects, and shutdown can lose messages. Clients cannot infer
remote delivery or replay safety from queue acceptance alone.
