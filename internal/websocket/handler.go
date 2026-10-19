package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

// Handler decodes and dispatches WebSocket messages.
// It is safe for concurrent use when its Service is.
type Handler struct {
	// shortcut: serialize in-memory transport work through queue acceptance;
	// use per-game dispatch locks if throughput requires concurrent games.
	dispatchMu   sync.Mutex
	gameService  gameplay.Service
	gameSessions *gameSessions
	admission    admission
	matchmaking  sync.WaitGroup
}

// newHandler creates a WebSocket message handler backed by gameService.
func newHandler(gameService gameplay.Service, sessions *gameSessions) *Handler {
	return &Handler{
		gameService:  gameService,
		gameSessions: sessions,
		admission:    admission{gameService: gameService, sessions: sessions},
	}
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
	_ = h.gameSessions.broadcastState(ctx, nil, "", state)
}

// Publish orders an automatic update with commands, admission, and disconnects.
func (h *Handler) Publish(ctx context.Context, state gameplay.GameSnapshot) {
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()
	_ = h.gameSessions.broadcastState(ctx, nil, "", state)
}

// Handle serializes a client command through state publication and queue acceptance.
func (h *Handler) Handle(ctx context.Context, client *Session, raw json.RawMessage) error {
	var message protocol.ClientEnvelope
	if err := json.Unmarshal(raw, &message); err != nil {
		return sendError(ctx, client, "", protocol.ErrorInvalidMessage, "message must be valid JSON")
	}
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()

	switch message.Type {
	case protocol.ClientCreateGame:
		return h.admission.create(ctx, client, message)
	case protocol.ClientJoinGame:
		return h.admission.join(ctx, client, message)
	case protocol.ClientResumeGame:
		return h.admission.resume(ctx, client, message)
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
		return sendError(ctx, client, message.RequestID, protocol.ErrorUnknownType, fmt.Sprintf("unknown message type %q", message.Type))
	}
}

func (h *Handler) handleMakeMove(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeMove(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	state, err := h.gameService.MakeMove(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.gameSessions.broadcastState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleResign(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeResign(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	state, err := h.gameService.Resign(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.gameSessions.broadcastState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleOfferDraw(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeOfferDraw(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	state, err := h.gameService.OfferDraw(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.gameSessions.broadcastState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleDrawResponse(ctx context.Context, client *Session, message protocol.ClientEnvelope, respond func(context.Context, gameplay.DrawOfferResponseCommand) (gameplay.GameSnapshot, error)) error {
	command, err := decodeDrawResponse(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	state, err := respond(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.gameSessions.broadcastState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleMatchmaking(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.EnterMatchmakingPayload](message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.matchmaking payload")
	}
	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid time control preset")
	}
	if err = h.gameSessions.Hold(client); err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}

	ticket, err := h.gameService.EnterMatchmaking(ctx, gameplay.EnterMatchmakingCommand{ProfileID: client.profileID, Initial: initial, Increment: increment})
	if err != nil {
		h.gameSessions.Release(client)
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerMatchmakingEntered, RequestID: message.RequestID, Payload: protocol.EnterMatchmakingPayload{TimeControl: request.TimeControl}}); err != nil {
		h.gameSessions.Release(client)
		return err
	}
	// The waiter owns the reservation until it is matched or canceled.
	h.matchmaking.Add(1)
	go func() {
		defer h.matchmaking.Done()
		// Service failures and queue pressure are logged where they occur.
		_ = h.awaitMatch(ctx, client, message.RequestID, ticket)
	}()
	return nil
}

func (h *Handler) awaitMatch(ctx context.Context, client *Session, requestID string, ticket gameplay.MatchTicket) error {
	var result gameplay.GameAssignment
	var matched bool
	select {
	case <-ctx.Done():
	case match, ok := <-ticket.Result:
		result = match
		matched = ok
	}
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()
	defer h.gameSessions.Release(client)
	if ctx.Err() != nil || !matched {
		return nil
	}

	state, err := h.gameService.GameState(ctx, result.GameID)
	if err != nil {
		return sendApplicationError(ctx, client, requestID, err)
	}
	return h.admission.admit(ctx, client, requestID, protocol.ServerMatchFound, mapColorFromServer(result.Color), state)
}
