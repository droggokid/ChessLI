package websocket

import (
	"context"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
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
	defer h.gameSessions.Release(client)

	result, err := h.gameService.CreatePrivateGame(ctx, gameplay.CreatePrivateCommand{
		ProfileID: client.profileID, Initial: initial, Increment: increment, ColorPreference: colorPreference,
	})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}

	if err := h.gameSessions.Add(result.GameID, client); err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameCreated,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: mapColorFromServer(result.Color)},
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
	defer h.gameSessions.Release(client)

	result, err := h.gameService.JoinPrivateGame(ctx, gameplay.JoinPrivateCommand{ProfileID: client.profileID, GameID: request.GameID})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	// Joining has committed; fetching and publishing its state survives source cancellation.
	peerCtx := context.WithoutCancel(ctx)
	state, err := h.gameService.GameState(peerCtx, result.GameID)
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	defer func() {
		h.gameSessions.BroadcastPeers(peerCtx, state.GameID, client, h.initialState(state))
		if state.Outcome != game.OutcomeNone {
			h.gameSessions.RemoveGame(state.GameID)
		}
	}()
	if err := h.gameSessions.Add(result.GameID, client); err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: mapColorFromServer(result.Color)},
	}); err != nil {
		return err
	}
	return client.Send(ctx, h.initialState(state))
}

func (h *Handler) handleResumeGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.ResumeGamePayload](message)
	if err != nil || request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.resume payload")
	}
	if err = h.gameSessions.Hold(client); err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}
	defer h.gameSessions.Release(client)

	state, err := h.gameService.ResumeGame(ctx, gameplay.ResumeGameCommand{GameID: request.GameID, ProfileID: client.profileID})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}

	// ResumeGame has already verified that the profile occupies one of the seats.
	color := protocol.ColorBlack
	if client.profileID == state.WhiteProfileID {
		color = protocol.ColorWhite
	}
	return h.admitGame(ctx, client, message.RequestID, protocol.ServerGameResumed, color, state)
}

// Finished games are read-only; active admission publishes presence even if the ack fails.
func (h *Handler) admitGame(ctx context.Context, client *Session, requestID string, responseType protocol.ServerMessageType, color protocol.Color, state gameplay.GameSnapshot) error {
	active := state.Outcome == game.OutcomeNone
	if active {
		if err := h.gameSessions.Add(state.GameID, client); err != nil {
			return h.sendApplicationError(ctx, client, requestID, err)
		}
		defer func() {
			h.gameSessions.BroadcastPeers(context.WithoutCancel(ctx), state.GameID, client, protocol.ServerEnvelope{Type: protocol.ServerGameState, Payload: h.gameStatePayload(state)})
		}()
	} else {
		_ = h.broadcastGameState(context.WithoutCancel(ctx), nil, "", state)
	}

	if err := client.Send(ctx, protocol.ServerEnvelope{
		Type:      responseType,
		RequestID: requestID,
		Payload:   protocol.GameCreatedPayload{GameID: state.GameID, Color: color},
	}); err != nil {
		return err
	}
	return client.Send(ctx, h.initialState(state))
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

	ticket, err := h.gameService.EnterMatchmaking(ctx, gameplay.EnterMatchmakingCommand{ProfileID: client.profileID, Initial: initial, Increment: increment})
	if err != nil {
		h.gameSessions.Release(client)
		return h.sendApplicationError(ctx, client, message.RequestID, err)
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
		return h.sendApplicationError(ctx, client, requestID, err)
	}
	return h.admitGame(ctx, client, requestID, protocol.ServerMatchFound, mapColorFromServer(result.Color), state)
}
