package websocket

import (
	"context"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

const sessionUnavailableMessage = "session is already in a game or matchmaking"

// admission coordinates seating and acknowledgements under Handler's dispatch mutex.
// It owns no goroutines or locks; the handler joins matchmaking waiters.
type admission struct {
	gameService gameplay.Service
	sessions    *gameSessions
}

func (a *admission) create(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeCreateGame(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	if err = a.sessions.Hold(client); err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}
	defer a.sessions.Release(client)

	result, err := a.gameService.CreatePrivateGame(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}

	if err := a.sessions.Add(result.GameID, client); err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameCreated,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: mapColorFromServer(result.Color)},
	})
}

func (a *admission) join(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeJoinGame(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	if err = a.sessions.Hold(client); err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}
	defer a.sessions.Release(client)

	result, err := a.gameService.JoinPrivateGame(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	// Joining has committed; fetching and publishing its state survives source cancellation.
	peerCtx := context.WithoutCancel(ctx)
	state, err := a.gameService.GameState(peerCtx, result.GameID)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	defer func() {
		a.sessions.BroadcastPeers(peerCtx, state.GameID, client, a.sessions.initialState(state))
		if state.Outcome != game.OutcomeNone {
			a.sessions.RemoveGame(state.GameID)
		}
	}()
	if err := a.sessions.Add(result.GameID, client); err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}
	if err = client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload:   protocol.GameCreatedPayload{GameID: result.GameID, Color: mapColorFromServer(result.Color)},
	}); err != nil {
		return err
	}
	return client.Send(ctx, a.sessions.initialState(state))
}

func (a *admission) resume(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	command, err := decodeResumeGame(client.profileID, message)
	if err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, err.Error())
	}

	if err = a.sessions.Hold(client); err != nil {
		return sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, sessionUnavailableMessage)
	}
	defer a.sessions.Release(client)

	state, err := a.gameService.ResumeGame(ctx, command)
	if err != nil {
		return sendApplicationError(ctx, client, message.RequestID, err)
	}

	// ResumeGame has already verified that the profile occupies one of the seats.
	color := protocol.ColorBlack
	if client.profileID == state.WhiteProfileID {
		color = protocol.ColorWhite
	}
	return a.admit(ctx, client, message.RequestID, protocol.ServerGameResumed, color, state)
}

// Finished games are read-only; active admission publishes presence even if the ack fails.
func (a *admission) admit(ctx context.Context, client *Session, requestID string, responseType protocol.ServerMessageType, color protocol.Color, state gameplay.GameSnapshot) error {
	active := state.Outcome == game.OutcomeNone
	if active {
		if err := a.sessions.Add(state.GameID, client); err != nil {
			return sendApplicationError(ctx, client, requestID, err)
		}
		defer func() {
			a.sessions.BroadcastPeers(context.WithoutCancel(ctx), state.GameID, client, protocol.ServerEnvelope{Type: protocol.ServerGameState, Payload: a.sessions.statePayload(state)})
		}()
	} else {
		_ = a.sessions.broadcastState(context.WithoutCancel(ctx), nil, "", state)
	}

	if err := client.Send(ctx, protocol.ServerEnvelope{
		Type:      responseType,
		RequestID: requestID,
		Payload:   protocol.GameCreatedPayload{GameID: state.GameID, Color: color},
	}); err != nil {
		return err
	}
	return client.Send(ctx, a.sessions.initialState(state))
}
