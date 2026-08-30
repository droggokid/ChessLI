package gameplay

import (
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type CreateResult struct {
	GameID identity.GameID
	Color  chess.Color
}

type JoinResult struct {
	GameID identity.GameID
	Color  chess.Color
}

type MatchResult struct {
	GameID identity.GameID
	Color  chess.Color
}

type MatchTicket struct {
	Result <-chan MatchResult
}
