package websocket

import (
	"context"
	"fmt"
	"log/slog"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

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

	if err = h.gameSessions.Hold(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}

	result, err := h.gameService.CreatePrivateGame(ctx, gameplay.CreatePrivateCommand{
		ProfileID: client.profile.ID, Initial: initial, Increment: increment, ColorPreference: colorPreference,
	})
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}

	color, err := h.completeGameAdmission(client, result.GameID, result.Color)
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameCreated,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: color},
	})
}

func (h *Handler) handleJoinGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.JoinGamePayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.join payload")
	}
	if request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "gameId is required")
	}
	if err = h.gameSessions.Hold(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}

	result, err := h.gameService.JoinPrivateGame(ctx, gameplay.JoinPrivateCommand{ProfileID: client.profile.ID, GameID: request.GameID})
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}
	color, err := h.completeGameAdmission(client, result.GameID, result.Color)
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: color},
	}); err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}

	state, err := h.gameService.GameState(ctx, result.GameID)
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	if state.Outcome != game.OutcomeNone {
		defer h.gameSessions.RemoveGame(state.GameID)
	}
	return h.gameSessions.Broadcast(ctx, result.GameID, client, h.initialState(state))
}

func (h *Handler) handleResumeGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.ResumeGamePayload](message)
	if err != nil || request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.resume payload")
	}
	if err = h.gameSessions.Hold(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}

	state, err := h.gameService.ResumeGame(ctx, gameplay.ResumeGameCommand{GameID: request.GameID, ProfileID: client.profile.ID})
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}

	var color protocol.Color
	switch client.profile.ID {
	case state.WhiteProfileID:
		color = protocol.ColorWhite
	case state.BlackProfileID:
		color = protocol.ColorBlack
	default:
		h.gameSessions.Release(client)
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInternal, "internal server error")
	}

	active := state.Outcome == game.OutcomeNone
	if active {
		if err = h.gameSessions.Add(state.GameID, client); err != nil {
			h.gameSessions.Release(client)
			return h.sendApplicationError(ctx, client, message.RequestID, err)
		}
	} else {
		h.gameSessions.Release(client)
		_ = h.broadcastGameState(ctx, nil, "", state)
	}

	if err = client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameResumed,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: state.GameID, Color: color},
	}); err != nil {
		return err
	}
	if err = client.Send(ctx, h.initialState(state)); err != nil {
		return err
	}
	if active {
		h.gameSessions.BroadcastPeers(ctx, state.GameID, client, protocol.ServerEnvelope{Type: protocol.ServerGameState, Payload: h.gameStatePayload(state)})
	}
	return nil
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
	if err = h.gameSessions.Hold(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}

	ticket, err := h.gameService.EnterMatchmaking(ctx, gameplay.NewEnterMatchmakingCommand(client.profile.ID, initial, increment))
	if err != nil {
		return h.releaseAndSendApplicationError(ctx, client, message.RequestID, err)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerMatchmakingEntered, RequestID: message.RequestID, Payload: protocol.EnterMatchmakingPayload{TimeControl: request.TimeControl}}); err != nil {
		h.gameSessions.Release(client)
		return err
	}
	go h.awaitMatch(ctx, client, message.RequestID, ticket)
	return nil
}

func (h *Handler) awaitMatch(ctx context.Context, client *Session, requestID string, ticket gameplay.MatchTicket) {
	var result gameplay.GameAssignment
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
	h.dispatchMu.Lock()
	defer h.dispatchMu.Unlock()
	if err := ctx.Err(); err != nil {
		h.gameSessions.Release(client)
		return
	}

	color, err := mapColorFromServer(result.Color)
	if err != nil {
		h.gameSessions.Release(client)
		if sendErr := h.sendError(ctx, client, requestID, protocol.ErrorInternal, "internal server error"); sendErr != nil {
			slog.Warn("send matchmaking color error", "error", sendErr)
		}
		return
	}
	state, err := h.gameService.GameState(ctx, result.GameID)
	if err != nil {
		h.gameSessions.Release(client)
		if sendErr := h.sendApplicationError(ctx, client, requestID, err); sendErr != nil {
			slog.Warn("send matchmaking state error", "error", sendErr)
		}
		return
	}
	active := state.Outcome == game.OutcomeNone
	if active {
		if err = h.gameSessions.Add(result.GameID, client); err != nil {
			h.gameSessions.Release(client)
			slog.Warn("register matched session", "game_id", result.GameID, "error", err)
			return
		}
	} else {
		h.gameSessions.Release(client)
		_ = h.broadcastGameState(ctx, nil, "", state)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerMatchFound, RequestID: requestID, Payload: protocol.GameCreatedPayload{GameID: result.GameID, Color: color}}); err != nil {
		slog.Warn("send matchmaking result", "error", err)
		return
	}

	if err = client.Send(ctx, h.initialState(state)); err != nil {
		slog.Warn("send matched game initial state", "game_id", result.GameID, "error", err)
	}
	if active {
		h.gameSessions.BroadcastPeers(context.WithoutCancel(ctx), state.GameID, client, protocol.ServerEnvelope{Type: protocol.ServerGameState, Payload: h.gameStatePayload(state)})
	}
}

func (h *Handler) completeGameAdmission(client *Session, gameID identity.GameID, serviceColor chess.Color) (protocol.Color, error) {
	color, err := mapColorFromServer(serviceColor)
	if err != nil {
		return "", fmt.Errorf("invalid service color %v", serviceColor)
	}
	if err = h.gameSessions.Add(gameID, client); err != nil {
		return "", err
	}
	return color, nil
}
