package gameplay

import (
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

const drawOfferCooldown = 2 * time.Minute

type DrawOffer struct {
	OfferID   identity.DrawOfferID
	GameID    identity.GameID
	OfferedBy identity.ProfileID
	CreatedAt time.Time
}

type drawOfferState struct {
	pending       *DrawOffer
	lastOfferedAt map[identity.ProfileID]time.Time
}

func (g *Game) OfferDraw(command OfferDrawCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	if err := g.validateDrawParticipantLocked(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if g.drawOffers.pending != nil {
		return GameSnapshot{}, ErrDrawOfferPending
	}
	if lastOfferedAt, ok := g.drawOffers.lastOfferedAt[command.ProfileID]; ok && now.Sub(lastOfferedAt) < drawOfferCooldown {
		return GameSnapshot{}, ErrDrawOfferCooldown
	}

	offer := DrawOffer{
		OfferID:   identity.NewDrawOfferID(),
		GameID:    g.ID,
		OfferedBy: command.ProfileID,
		CreatedAt: now,
	}
	g.drawOffers.pending = &offer
	g.drawOffers.lastOfferedAt[command.ProfileID] = now

	return g.snapshotLocked(now), nil
}

func (g *Game) AcceptDraw(command DrawOfferResponseCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	if err := g.validateDrawParticipantLocked(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.validateDrawResponseLocked(command); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.engine.Draw(chess.DrawOffer); err != nil {
		return GameSnapshot{}, err
	}

	g.syncOutcomeFromEngineLocked()
	g.clock.stop(now, g.engine.Position().Turn())
	g.version++
	g.clearPendingDrawOfferLocked()

	return g.snapshotLocked(now), nil
}

func (g *Game) DeclineDraw(command DrawOfferResponseCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	if err := g.validateDrawParticipantLocked(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.validateDrawResponseLocked(command); err != nil {
		return GameSnapshot{}, err
	}

	g.clearPendingDrawOfferLocked()
	return g.snapshotLocked(now), nil
}

func (g *Game) validateDrawParticipantLocked(profileID identity.ProfileID, now time.Time) error {
	if err := g.validateReady(); err != nil {
		return err
	}
	if g.expireLocked(now) || g.outcome != chess.NoOutcome {
		return ErrGameFinished
	}
	if profileID != g.WhiteProfileID && profileID != g.BlackProfileID {
		return ErrNotParticipant
	}
	return nil
}

func (g *Game) validateDrawResponseLocked(command DrawOfferResponseCommand) error {
	offer := g.drawOffers.pending
	if offer == nil {
		return ErrDrawOfferNotFound
	}
	if offer.OfferID != command.OfferID {
		return ErrStaleDrawOffer
	}
	if offer.OfferedBy == command.ProfileID {
		return ErrCannotRespondToOwnDrawOffer
	}
	return nil
}

func (g *Game) clearPendingDrawOfferLocked() {
	g.drawOffers.pending = nil
}
