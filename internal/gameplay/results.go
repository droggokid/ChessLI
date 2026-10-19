package gameplay

import (
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
