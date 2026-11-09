# ADR 0003: Outgoing Message Delivery

Status: Accepted.

## Decision

`Session.Send` attempts a non-blocking enqueue into a bounded outgoing queue.
Success confirms queue acceptance only. A stopped session, canceled caller,
or full queue returns the corresponding error. Delivery racing with shutdown
may be lost.

Handler serializes game operations, membership changes, and queue acceptance.
An older active snapshot must not follow a terminal snapshot, and admission
must not register a finished game as active. Actual socket writes happen
outside that lock.

After an action commits, peer enqueue attempts proceed even if the acting
player's response fails or its request is canceled. The source error is returned
after those attempts. Join publishes initial state to the peer; resume and
matchmaking publish presence. Terminal publication releases live membership.
[Request IDs](0004-protocol-compatibility.md) belong only to the initiating client.

## Rationale

Bounded, non-blocking queues keep a slow client from blocking gameplay. They
permit message loss; neither an enqueue nor a socket write proves peer receipt.
Timeout publication runs asynchronously to avoid re-entering the handler's
dispatch lock during a service call. Clients recover through authoritative
state instead of assuming an unacknowledged action can be replayed safely.
