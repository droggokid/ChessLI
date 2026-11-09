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

// DrawOffer identifies a pending offer and the participant who made it.
type DrawOffer struct {
	OfferID   identity.DrawOfferID
	OfferedBy identity.ProfileID
}
