package protocol

import (
	"ChessLI/internal/identity"
)

type CreateGamePayload struct {
	TimeControl TimeControlPreset `json:"timeControl"`
	Color       ColorPreference   `json:"color"`
}

type JoinGamePayload struct {
	GameID identity.GameID `json:"gameId"`
}

type EnterMatchmakingPayload struct {
	TimeControl TimeControlPreset `json:"timeControl"`
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

type TimeControlPreset string

const (
	TimeControlBullet1Plus0     TimeControlPreset = "1+0"
	TimeControlBullet1Plus1     TimeControlPreset = "1+1"
	TimeControlBullet2Plus1     TimeControlPreset = "2+1"
	TimeControlBlitz3Plus0      TimeControlPreset = "3+0"
	TimeControlBlitz3Plus2      TimeControlPreset = "3+2"
	TimeControlBlitz5Plus0      TimeControlPreset = "5+0"
	TimeControlRapid10Plus0     TimeControlPreset = "10+0"
	TimeControlRapid10Plus5     TimeControlPreset = "10+5"
	TimeControlRapid15Plus10    TimeControlPreset = "15+10"
	TimeControlClassical30Plus0 TimeControlPreset = "30+0"
)
