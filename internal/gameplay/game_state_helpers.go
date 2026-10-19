package gameplay

import (
	"ChessLI/internal/identity"
	"github.com/corentings/chess/v2"
)

func colorFromProfileID(profileID, whiteProfileID, blackProfileID identity.ProfileID) (chess.Color, error) {
	switch profileID {
	case whiteProfileID:
		return chess.White, nil
	case blackProfileID:
		return chess.Black, nil
	default:
		return chess.NoColor, ErrNotParticipant
	}
}
