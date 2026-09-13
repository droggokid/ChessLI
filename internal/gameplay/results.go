package gameplay

import (
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type GameAssignment struct {
	GameID identity.GameID
	Color  chess.Color
}

type MatchTicket struct {
	Result <-chan GameAssignment
}
