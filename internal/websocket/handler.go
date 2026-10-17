package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

const sessionUnavailableMessage = "session is already in a game or matchmaking"

// Handler decodes and dispatches WebSocket messages.
// It is safe for concurrent use when its Service is.
type Handler struct {
	// shortcut: serialize in-memory transport work through queue acceptance;
	// use per-game dispatch locks if throughput requires concurrent games.
	dispatchMu   sync.Mutex
	gameService  gameplay.Service
	gameSessions *gameSessions
	matchmaking  sync.WaitGroup
}

// newHandler creates a WebSocket message handler backed by gameService.
func newHandler(gameService gameplay.Service, sessions *gameSessions) *Handler {
	return &Handler{gameService: gameService, gameSessions: sessions}
}

// Close joins matchmaking waiters after all connection operations have stopped.
// Their contexts must already be canceled; Close does not cancel them.
func (h *Handler) Close() {
	h.matchmaking.Wait()
}

// Disconnect removes membership and publishes the remaining player's view.
func (h *Handler) Disconnect(ctx context.Context, client *Session) {
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()
	gameID := h.gameSessions.Remove(client)
	if gameID == "" {
		return
	}
	shuttingDown := ctx.Err() != nil
	slog.Info("player disconnected from game", "event", "game.player_disconnected",
		"game_id", gameID, "profile_id", client.profileID, "server_shutdown", shuttingDown)
	if shuttingDown {
		return
	}
	state, err := h.gameService.GameState(ctx, gameID)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("fetch disconnected game state", "event", "disconnect.state_failed",
				"game_id", gameID, "profile_id", client.profileID, "error", err)
		}
		return
	}
	_ = h.broadcastGameState(ctx, nil, "", state)
}

// Publish orders an automatic update with commands, admission, and disconnects.
func (h *Handler) Publish(ctx context.Context, state gameplay.GameSnapshot) {
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()
	_ = h.broadcastGameState(ctx, nil, "", state)
}

// Handle serializes a client command through state publication and queue acceptance.
func (h *Handler) Handle(ctx context.Context, client *Session, raw json.RawMessage) error {
	var message protocol.ClientEnvelope
	if err := json.Unmarshal(raw, &message); err != nil {
		return h.sendError(ctx, client, "", protocol.ErrorInvalidMessage, "message must be valid JSON")
	}
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()

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

func (h *Handler) sendApplicationError(ctx context.Context, client *Session, requestID string, cause error) error {
	code, publicMessage := mapApplicationError(cause)
	if code == protocol.ErrorInternal && ctx.Err() == nil && !errors.Is(cause, protocol.ErrSessionNotRunning) {
		slog.Error("game request failed", "event", "request.failed", "profile_id", client.profileID,
			"request_id", requestID, "error_code", code, "error", cause)
	}
	return h.sendError(ctx, client, requestID, code, publicMessage)
}

func (h *Handler) sendError(ctx context.Context, client *Session, requestID string, code protocol.ErrorCode, message string) error {
	return client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerError, RequestID: requestID, Payload: protocol.ErrorPayload{Code: code, Message: message}})
}
