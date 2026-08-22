package protocol

import (
	"ChessLI/internal/identity"
)

type CreateGamePayload struct {
	TimeControl *TimeControl    `json:"timeControl"`
	Color       ColorPreference `json:"color"`
}

type JoinGamePayload struct {
	GameID identity.GameID `json:"gameId"`
}

type EnterMatchmakingPayload struct {
	TimeControl *TimeControl `json:"timeControl"`
}

type MatchFoundPayload struct {
	GameID identity.GameID `json:"gameId"`
	Color  Color           `json:"color"`
}

type MoveNotation string

const (
	MoveNotationUCI MoveNotation = "uci"
	MoveNotationSAN MoveNotation = "san"
	MoveNotationLAN MoveNotation = "lan"
)

type MovePayload struct {
	GameID          identity.GameID `json:"gameId"`
	Move            string          `json:"move"`
	Notation        MoveNotation    `json:"notation"`
	ExpectedVersion *uint64         `json:"expectedVersion"`
}

type TimeControl struct {
	InitialMilliseconds   int64 `json:"initialMilliseconds"`
	IncrementMilliseconds int64 `json:"incrementMilliseconds"`
}
