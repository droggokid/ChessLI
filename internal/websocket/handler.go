package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

const sessionUnavailableMessage = "session is already in a game or matchmaking"

// Handler decodes and dispatches WebSocket messages.
// It is safe for concurrent use when its Service is.
type Handler struct {
	gameService  gameplay.Service
	gameSessions *GameSessions
}

// NewHandler creates a WebSocket message handler backed by gameService.
func NewHandler(gameService gameplay.Service, sessions *GameSessions) *Handler {
	return &Handler{gameService: gameService, gameSessions: sessions}
}

// Handle decodes and dispatches a client message.
func (h *Handler) Handle(ctx context.Context, client *Session, raw json.RawMessage) error {
	var message protocol.ClientEnvelope
	if err := json.Unmarshal(raw, &message); err != nil {
		return h.sendError(ctx, client, "", protocol.ErrorInvalidMessage, "message must be valid JSON")
	}

	switch message.Type {
	case protocol.ClientCreateGame:
		return h.handleCreateGame(ctx, client, message)
	case protocol.ClientJoinGame:
		return h.handleJoinGame(ctx, client, message)
	case protocol.ClientResumeGame:
		return h.handleResumeGame(ctx, client, message)
	case protocol.ClientMakeMove:
		return h.handleMakeMove(ctx, client, message)
	case protocol.ClientEnterMatchmaking:
		return h.handleMatchmaking(ctx, client, message)
	case protocol.ClientResign:
		return h.handleResign(ctx, client, message)
	case protocol.ClientOfferDraw:
		return h.handleOfferDraw(ctx, client, message)
	case protocol.ClientAcceptDraw:
		return h.handleDrawResponse(ctx, client, message, h.gameService.AcceptDraw)
	case protocol.ClientDeclineDraw:
		return h.handleDrawResponse(ctx, client, message, h.gameService.DeclineDraw)
	default:
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorUnknownType, fmt.Sprintf("unknown message type %q", message.Type))
	}
}

func (h *Handler) releaseAndSendApplicationError(ctx context.Context, client *Session, requestID string, cause error) error {
	h.gameSessions.Release(client)
	return h.sendApplicationError(ctx, client, requestID, cause)
}

func (h *Handler) sendApplicationError(ctx context.Context, client *Session, requestID string, cause error) error {
	code, publicMessage := mapApplicationError(cause)
	return h.sendError(ctx, client, requestID, code, publicMessage)
}

func (h *Handler) sendError(ctx context.Context, client *Session, requestID string, code protocol.ErrorCode, message string) error {
	return client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerError, RequestID: requestID, Payload: protocol.ErrorPayload{Code: code, Message: message}})
}
