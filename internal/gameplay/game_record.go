package gameplay

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

// completedGameRecord is the gameplay record for a finished game, independent
// of the chess engine and persistence models. Moves are ordered by ply.
type completedGameRecord struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Moves          []recordedMove
	Outcome        domain.Outcome
	Termination    domain.Termination
	FinalFEN       string
	CompletedAt    time.Time
}

// recordedMove holds both clock balances immediately after the accepted move,
// including the mover's Fischer increment.
type recordedMove struct {
	Ply            int
	UCI            string
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
}

// moveClockBalances follows the engine's move history in the same order.
type moveClockBalances struct {
	white time.Duration
	black time.Duration
}
