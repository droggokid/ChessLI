package game

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

type CompletedGame struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Outcome        domain.Outcome
	Termination    domain.Termination
	FinalFEN       string
	CompletedAt    time.Time
	Moves          []Move
}
