package gameplay

import (
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type CreateResult struct {
	GameID identity.GameID `json:"gameId"`
	Color  chess.Color     `json:"color"`
}

type JoinResult struct {
	GameID identity.GameID `json:"gameId"`
	Color  chess.Color     `json:"color"`
}

type MatchResult struct {
	GameID identity.GameID `json:"gameId"`
	Color  chess.Color     `json:"color"`
}

type MatchTicket struct {
	Result <-chan MatchResult
}
