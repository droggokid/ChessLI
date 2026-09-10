package protocol

import (
	"ChessLI/internal/identity"
)

type GameCreatedPayload struct {
	GameID identity.GameID `json:"gameId"`
	Color  Color           `json:"color"`
}

type GameJoinedPayload struct {
	GameID identity.GameID `json:"gameId"`
	Color  Color           `json:"color"`
}

type GameStatePayload struct {
	GameID  identity.GameID `json:"gameId"`
	FEN     string          `json:"fen"`
	Status  GameStatus      `json:"status"`
	Version uint64          `json:"version"`

	White *PlayerState `json:"white,omitempty"`
	Black *PlayerState `json:"black,omitempty"`

	LastMove         string            `json:"lastMove,omitempty"`
	Outcome          *GameOutcome      `json:"outcome,omitempty"`
	PendingDrawOffer *DrawOfferPayload `json:"pendingDrawOffer,omitempty"`
}

type DrawOfferPayload struct {
	OfferID   identity.DrawOfferID `json:"offerId"`
	OfferedBy Color                `json:"offeredBy"`
}

type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}
