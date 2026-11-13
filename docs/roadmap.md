# Roadmap

These are future directions, not implemented features. Current behavior and
commands are in [README](../README.md).

## Durable identities and history

Game and profile repositories are injected into gameplay. Connections validate
existing development profiles, and completed games are saved with their moves and
final clocks independently of final-state delivery. Saves have three bounded
attempts; failed or pending saves have no crash recovery. Authentication,
authorization, stronger retention, and exposing history after restart remain
future work. Unfinished games remain in memory only.

The first history milestone is a real dashboard, searchable game history, and
stepping through a selected game's moves. Dashboard statistics and History are
currently placeholders.

## Imports and analysis

Add PGN uploads before direct Lichess or Chess.com imports. Preserve game source,
external IDs, and the original move sequence. Provider integrations need account
linking and rate-limit handling.

Derive result summaries, matchup history, and opening frequency from stored games.
Run engine analysis asynchronously and store evaluations, mistakes, and engine
settings separately from the immutable game. Start with job IDs and status
polling; add subscriptions when progress updates justify them. Authentication,
job lifetime, engine execution limits, and analysis methods remain open.

## Client and API direction

Keep client and server as separate executables in this repository. The installed
terminal client should connect to a hosted deployment, with a configurable
server address. Packaging, release distribution, and supported platforms remain
open; a future web client would use the same deployment's APIs.

GraphQL over HTTPS is the preferred direction for dashboard, history, imports,
and analysis, alongside the existing live-game WebSocket API. It is not
implemented; schema, tooling, pagination, and query-cost limits need design.
Business rules stay outside resolvers and unrelated dashboard operations stay
outside `gameplay.Service`.

The deployment should own provider imports, stored engine analysis, and shared
insights. Clients own navigation, rendering, local file selection, and responsive
exploration of downloaded moves. A local engine is optional future work.

## Other open decisions

Ratings, rematches, spectators, friendship/blocking, player preferences,
disconnect claims or grace periods, and active-game recovery after restart
remain separate features. Revisit parallel game execution or replica coordination
when deployment requirements and measured contention warrant it.
