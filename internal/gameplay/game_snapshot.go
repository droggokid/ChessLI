package gameplay

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

type GameSnapshot struct {
	GameID           identity.GameID
	FEN              string
	Version          uint64
	WhiteProfileID   identity.ProfileID
	BlackProfileID   identity.ProfileID
	WhiteRemaining   time.Duration
	BlackRemaining   time.Duration
	LastMoveSAN      string
	Outcome          domain.Outcome
	Termination      domain.Termination
	PendingDrawOffer *DrawOffer
}

// snapshot returns a consistent copy of the game's authoritative state.
// It takes g.mu exclusively because observing an expired clock updates that state.
func (g *game) snapshot() GameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	g.expireLocked(now)
	return g.snapshotLocked(now)
}

func (g *game) snapshotLocked(now time.Time) GameSnapshot {
	whiteRemaining, blackRemaining := g.clock.remaining(now, g.engine.Position().Turn())
	var pendingDrawOffer *DrawOffer
	if g.drawOffers.pending != nil {
		pendingDrawOffer = new(*g.drawOffers.pending)
	}

	return GameSnapshot{
		GameID:           g.id,
		FEN:              g.engine.FEN(),
		Version:          g.version,
		WhiteProfileID:   g.whiteProfileID,
		BlackProfileID:   g.blackProfileID,
		WhiteRemaining:   whiteRemaining,
		BlackRemaining:   blackRemaining,
		LastMoveSAN:      g.lastMoveSAN,
		Outcome:          g.outcome,
		Termination:      g.termination,
		PendingDrawOffer: pendingDrawOffer,
	}
}
