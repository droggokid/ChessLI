# ADR 0001: Core and Adapter Boundaries

Status: Accepted.

## Decision

`gameplay.Service` exposes named operations usable independently of WebSocket
and database implementations. Gameplay and shared domain types do not depend
on concrete transports or storage adapters. The executable wires them together.

WebSocket decoders and mappers translate wire payloads into gameplay values.
Persistence packages own storage models and translate driver/JSON representations
internally. Shared identity and game outcome types cross these boundaries.
The chess library is an intentional gameplay dependency; its types do not appear
in the wire protocol.

## Rationale

Explicit boundaries keep rules testable without network or database access and
allow clients to share application behavior. They cost some mapping code.
Interfaces and mocks serve actual component contracts; package layout and helper
names are implementation choices.
