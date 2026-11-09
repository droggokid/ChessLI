package game

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

type Summary struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Outcome        domain.Outcome
	Termination    domain.Termination
	CompletedAt    time.Time
}
