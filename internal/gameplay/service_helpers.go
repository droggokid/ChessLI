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

func assignPrivateColors(profileID identity.ProfileID, creatorColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if creatorColor == chess.White {
		return profileID, ""
	}
	return "", profileID
}
