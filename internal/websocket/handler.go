package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

type Handler struct {
	gameService  gameplay.Service
	gameSessions *GameSessions
}

// NewHandler returns a WebSocket message handler backed by gameService.
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

	case protocol.ClientMakeMove:
		return h.handleMakeMove(ctx, client, message)

	case protocol.ClientEnterMatchmaking:
		return h.handleMatchmaking(ctx, client, message)

	case protocol.ClientResign:
		return h.handleResign(ctx, client, message)

	case protocol.ClientOfferDraw:
		return h.handleOfferDraw(ctx, client, message)

	case protocol.ClientAcceptDraw:
		return h.handleAcceptDraw(ctx, client, message)

	case protocol.ClientDeclineDraw:
		return h.handleDeclineDraw(ctx, client, message)

	default:
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorUnknownType, fmt.Sprintf("unknown message type %q", message.Type))
	}
}

func (h *Handler) handleCreateGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.CreateGamePayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.create payload")
	}

	colorPreference, err := mapColorPreference(request.Color)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid color preference")
	}

	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid time control preset")
	}

	if err = h.gameSessions.Reserve(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game or matchmaking")
	}

	command := gameplay.CreatePrivateCommand{
		ProfileID:       client.profileID,
		ColorPreference: colorPreference,
		Initial:         initial,
		Increment:       increment,
	}

	result, err := h.gameService.CreatePrivateGame(ctx, command)
	if err != nil {
		h.gameSessions.Release(client)
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	if err = h.gameSessions.Add(result.GameID, client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game or matchmaking")
	}

	color, err := mapColorFromServer(result.Color)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInternal, "internal server error")
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerGameCreated,
		RequestID: message.RequestID,
		Payload: protocol.GameCreatedPayload{
			GameID: result.GameID,
			Color:  color,
		},
	}

	return client.Send(ctx, envelope)
}

func (h *Handler) handleJoinGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.JoinGamePayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.join payload")
	}

	if request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "gameId is required")
	}

	if err = h.gameSessions.Reserve(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game or matchmaking")
	}

	command := gameplay.NewJoinPrivateCommand(client.profileID, request.GameID)

	serviceResult, err := h.gameService.JoinPrivateGame(ctx, command)
	if err != nil {
		h.gameSessions.Release(client)
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	if err = h.gameSessions.Add(serviceResult.GameID, client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game or matchmaking")
	}

	color, err := mapColorFromServer(serviceResult.Color)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInternal, "internal server error")
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload: protocol.GameJoinedPayload{
			GameID: serviceResult.GameID,
			Color:  color,
		},
	}
	if err = client.Send(ctx, envelope); err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	state, err := h.gameService.GameState(ctx, serviceResult.GameID)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	return h.gameSessions.Broadcast(ctx, serviceResult.GameID, client, h.initialState(state))
}

func (h *Handler) handleMatchmaking(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.EnterMatchmakingPayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.matchmaking payload")
	}

	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid time control preset")
	}

	if err = h.gameSessions.Queue(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game or matchmaking")
	}

	command := gameplay.NewEnterMatchmakingCommand(client.profileID, initial, increment)

	ticket, err := h.gameService.EnterMatchmaking(ctx, command)
	if err != nil {
		h.gameSessions.Release(client)
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerMatchmakingEntered,
		RequestID: message.RequestID,
		Payload:   protocol.EnterMatchmakingPayload{TimeControl: request.TimeControl},
	}

	if err = client.Send(ctx, envelope); err != nil {
		h.gameSessions.Release(client)
		return err
	}

	go h.awaitMatch(ctx, client, message.RequestID, ticket)

	return nil
}

func (h *Handler) awaitMatch(ctx context.Context, client *Session, requestID string, ticket gameplay.MatchTicket) {
	var result gameplay.MatchResult

	select {
	case <-ctx.Done():
		h.gameSessions.Release(client)
		return
	case match, ok := <-ticket.Result:
		if !ok {
			h.gameSessions.Release(client)
			return
		}

		result = match
	}

	if err := h.gameSessions.Add(result.GameID, client); err != nil {
		slog.Warn("register matched session", "game_id", result.GameID, "error", err)
		return
	}

	if err := h.gameSessions.WaitForPlayers(ctx, result.GameID, 2); err != nil {
		return
	}

	color, err := mapColorFromServer(result.Color)
	if err != nil {
		if sendErr := h.sendError(ctx, client, requestID, protocol.ErrorInternal, "internal server error"); sendErr != nil {
			slog.Warn("send matchmaking color error", "error", sendErr)
		}
		return
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerMatchFound,
		RequestID: requestID,
		Payload: protocol.MatchFoundPayload{
			GameID: result.GameID,
			Color:  color,
		},
	}
	if err = client.Send(ctx, envelope); err != nil {
		slog.Warn("send matchmaking result", "error", err)
		return
	}

	state, err := h.gameService.GameState(ctx, result.GameID)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		if sendErr := h.sendError(ctx, client, requestID, code, publicMessage); sendErr != nil {
			slog.Warn("send matchmaking state error", "error", sendErr)
		}

		return
	}

	if err = client.Send(ctx, h.initialState(state)); err != nil {
		slog.Warn("send matched game initial state", "game_id", result.GameID, "error", err)
	}
}

func (h *Handler) handleMakeMove(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.MovePayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.move payload")
	}

	if request.Move == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "move is required")
	}

	if request.ExpectedVersion == nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "expectedVersion is required")
	}

	moveNotation, err := mapMoveNotation(request.Notation)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "unsupported move notation")
	}

	command := gameplay.NewMoveCommand(request.GameID, client.profileID, request.Move, moveNotation, *request.ExpectedVersion)

	state, err := h.gameService.MakeMove(ctx, command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)
		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerGameState,
		RequestID: message.RequestID,
		Payload:   h.gameStatePayload(state),
	}

	return h.gameSessions.Broadcast(ctx, state.GameID, client, envelope)
}

func (h *Handler) handleResign(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
}

func (h *Handler) handleOfferDraw(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
}

func (h *Handler) handleAcceptDraw(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
}

func (h *Handler) handleDeclineDraw(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
}

func (h *Handler) sendError(ctx context.Context, client *Session, requestID string, code protocol.ErrorCode, message string) error {
	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerError,
		RequestID: requestID,
		Payload: protocol.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

func (h *Handler) sendNotImplemented(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendError(ctx, client, message.RequestID, protocol.ErrorNotImplemented, fmt.Sprintf("%s is not implemented", message.Type))
}
