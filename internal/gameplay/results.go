package gameplay

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// GameAssignment identifies a player's seat; successful assignments are White or Black.
type GameAssignment struct {
	GameID identity.GameID
	Color  chess.Color
}

// MatchTicket yields one assignment, or closes without one when canceled before matching.
type MatchTicket struct {
	Result <-chan GameAssignment
}

// CompletedGame is a detached record of a finished gameplay session.
type CompletedGame struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Outcome        domain.Outcome
	Termination    domain.Termination
	FinalFEN       string
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
	CompletedAt    time.Time
	Moves          []RecordedMove
}

type RecordedMove struct {
	Ply            int
	UCI            string
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
}
