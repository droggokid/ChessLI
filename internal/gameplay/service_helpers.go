package gameplay

import (
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/persistence"

	"github.com/corentings/chess/v2"
)

func completedGameDTO(record CompletedGame) persistence.CompletedGame {
	moves := make([]persistence.Move, len(record.Moves))
	for i, move := range record.Moves {
		moves[i] = persistence.Move{
			Ply:              move.Ply,
			UCI:              move.UCI,
			WhiteRemainingMS: move.WhiteRemaining.Milliseconds(),
			BlackRemainingMS: move.BlackRemaining.Milliseconds(),
		}
	}
	return persistence.CompletedGame{
		ID:               record.ID,
		WhiteProfileID:   record.WhiteProfileID,
		BlackProfileID:   record.BlackProfileID,
		InitialTimeMS:    record.Initial.Milliseconds(),
		IncrementMS:      record.Increment.Milliseconds(),
		Outcome:          record.Outcome,
		Termination:      record.Termination,
		FinalFEN:         record.FinalFEN,
		WhiteRemainingMS: record.WhiteRemaining.Milliseconds(),
		BlackRemainingMS: record.BlackRemaining.Milliseconds(),
		CompletedAt:      record.CompletedAt,
		Moves:            moves,
	}
}

func validateTimeControl(initial, increment time.Duration) error {
	if initial <= 0 || increment < 0 {
		return ErrInvalidTimeControl
	}
	return nil
}

func validateProfileID(profileID identity.ProfileID) error {
	if profileID == "" {
		return ErrInvalidProfileID
	}
	return nil
}

// assignColors seats both players using the first player's already chosen color.
func assignColors(first, second identity.ProfileID, firstColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if firstColor == chess.White {
		return first, second
	}
	return second, first
}
