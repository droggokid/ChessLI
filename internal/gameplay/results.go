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

type MoveResult struct {
	GameID  identity.GameID `json:"gameId"`
	FEN     string          `json:"fen"`
	Outcome chess.Outcome   `json:"outcome"`
	Method  chess.Method    `json:"method"`
}

type MatchResult struct {
	GameID identity.GameID
	Color  chess.Color
}

type MatchTicket struct {
	Result <-chan MatchResult
}
