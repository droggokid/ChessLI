package gameplay

import (
	"ChessLI/internal/identity"
	"github.com/corentings/chess/v2"
)

// ColorFromProfileID returns the profile's seat, or ErrNotParticipant if it has none.
func ColorFromProfileID(profileID, whiteProfileID, blackProfileID identity.ProfileID) (chess.Color, error) {
	switch profileID {
	case whiteProfileID:
		return chess.White, nil
	case blackProfileID:
		return chess.Black, nil
	default:
		return chess.NoColor, ErrNotParticipant
	}
}
