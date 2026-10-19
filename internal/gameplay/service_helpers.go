package gameplay

import (
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

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
