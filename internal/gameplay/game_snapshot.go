package gameplay

import (
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type GameSnapshot struct {
	GameID         identity.GameID
	FEN            string
	Version        uint64
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
	LastMoveSAN    string
	Outcome        chess.Outcome
	Termination    TerminationReason
}

// Snapshot returns a consistent copy of the game's current authoritative state.
func (g *Game) Snapshot() GameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	g.expireLocked(now)
	return g.snapshotLocked(now)
}

func (g *Game) snapshotLocked(now time.Time) GameSnapshot {
	whiteRemaining, blackRemaining := g.clock.remaining(now, g.engine.Position().Turn())

	return GameSnapshot{
		GameID:         g.ID,
		FEN:            g.engine.FEN(),
		Version:        g.version,
		WhiteProfileID: g.WhiteProfileID,
		BlackProfileID: g.BlackProfileID,
		WhiteRemaining: whiteRemaining,
		BlackRemaining: blackRemaining,
		LastMoveSAN:    g.lastMoveSAN,
		Outcome:        g.outcome,
		Termination:    g.termination,
	}
}
