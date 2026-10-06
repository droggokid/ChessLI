# Terminal Client and Dashboard Architecture

Planning notes from the client architecture discussion. These describe intended
direction, not an implemented client or API contract.

## Product direction

ChessLI is a terminal-first chess dashboard and analysis application, with live
play as one capability. The client should support browsing game history,
importing games from Lichess and Chess.com, reviewing and analysing games, and
deriving insights about how the player plays chess.

The terminal client is the current focus. A future web client should consume
the same deployment's APIs.

See [Imported Games and Dashboard Insights](persistence-foundation.md#future-imported-games-and-dashboard-insights)
for the existing sequence: PGN import before provider integrations; game
history, result summaries, and opening frequency before engine insights.

## Monorepo and distribution

- Keep client and server in the same repository, with separate executables.
- Players install the terminal client locally through a package manager, such
  as pacman, and connect to the hosted deployment. They do not need to run the
  server or its database.
- The client communicates through network APIs rather than calling server
  application services directly. An SSH-hosted interface is not the chosen
  delivery model.
- A Go client in `cmd/chessli` within the existing module is the suggested
  starting point. Independent binaries do not require separate Go modules.
- Default to the hosted deployment, with a server-address override for local
  development or another deployment.
- Client releases and server deployments update independently. API changes
  must account for players running older supported clients.

For Arch, `pacman -S chessli` requires publishing the package in a configured
package repository. A downloadable package can also be installed with
`pacman -U`. The distribution channel and supported platforms remain open.

## API direction

The preferred approach is GraphQL over HTTPS for dashboards, game history,
imports, and analysis, alongside the existing WebSocket API for live gameplay.
The GraphQL schema and implementation tooling have not been selected.

```text
Installed terminal client
  |-- GraphQL / HTTPS --> dashboard, games, imports, analysis
  |-- WebSocket -------> live gameplay commands and events
                         [same server deployment]
```

GraphQL fits the different views of related data: an overview requests recent
games and summary statistics, a library requests filtered game summaries, and
an analysis screen requests moves and evaluations for one game. A future web
client can request different selections from the same schema.

Multiple clients do not require GraphQL; REST remains viable. GraphQL's benefit
here is flexible access to related chess data. Its costs include maintaining
the schema and resolvers, bounding pagination, and controlling nested-query
cost. It does not solve replication, game ownership, or cross-replica delivery.

Both endpoints can live in the existing deployment. This direction does not
require a separate GraphQL service or replacing the live gameplay protocol.
Model the schema around chess capabilities rather than database tables or
terminal-specific screens. Keep business rules outside transport resolvers,
and keep unrelated dashboard operations out of `gameplay.Service`.

## Terminal experience

Make the dashboard the starting screen, with these main areas:

| Area | Purpose |
| --- | --- |
| Overview | Recent results, opening patterns, and available insights. |
| Games | Search and filter ChessLI and imported games; select one to review. |
| Analysis | Navigate moves, inspect evaluations and mistakes, explore variations. |
| Imports | Submit PGN, initiate provider imports, and inspect progress or failures. |
| Play | Matchmaking, private games, and resuming live play. |

A full-screen Go TUI is the suggested implementation, with Bubble Tea as a
candidate. Keep network handling separate from screen rendering. The exact
library, layout, and controls remain open.

## Analysis and import responsibilities

Following the existing product notes, the recommended split is:

- The deployment owns provider imports, stored engine analysis, and
  player-level insights so terminal and web clients see consistent results.
- The client owns navigation, rendering, local file selection, and responsive
  board exploration. Stepping through downloaded moves or exploring variations
  should not require a server request for every keypress.
- Local board interaction does not require shipping a local engine or
  duplicating the server's analysis pipeline.
- Derived evaluations and insights remain separate from the original game.
- Long-running import or analysis requests start asynchronous jobs and return
  identifiers. The suggested initial client behavior is polling job status
  while the relevant screen is open; add subscriptions when immediate progress
  updates justify them. Expensive analysis does not run inside the request
  that starts it.

Authentication, provider account linking, job lifecycle, engine execution
limits, and the specific analysis or inference methods need separate design.
No application or persistence implementation was assessed for these notes.

## Suggested first milestone

Install and launch the client, open a dashboard, browse game history, select a
game, and step through its moves. Use this slice to establish the first
dashboard API and terminal interaction patterns, then expand imports and
insights according to the existing product sequence.

Web implementation, replica coordination, and optional local/offline engine
analysis are outside this first milestone.

## References

- [GraphQL domain modelling](https://graphql.org/learn/thinking-in-graphs/)
- [GraphQL performance](https://graphql.org/learn/performance/)
- [Arch pacman manual](https://man.archlinux.org/man/pacman.8.en)
