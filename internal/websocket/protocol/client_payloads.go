package protocol

import (
	"ChessLI/internal/identity"
)

type CreateGamePayload struct {
	TimeControl *TimeControl    `json:"timeControl,omitempty"`
	Color       ColorPreference `json:"color,omitempty"`
}

type JoinGamePayload struct {
	GameID identity.GameID `json:"gameId"`
}

type EnterMatchmakingPayload struct {
	TimeControl *TimeControl `json:"timeControl,omitempty"`
}

type MovePayload struct {
	Move string `json:"move"`
}

type TimeControl struct {
	InitialMilliseconds   int64 `json:"initialMilliseconds"`
	IncrementMilliseconds int64 `json:"incrementMilliseconds"`
}
