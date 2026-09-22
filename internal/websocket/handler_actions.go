package websocket

import (
	"context"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

func (h *Handler) handleMakeMove(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.MovePayload](message)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.move payload")
	}
	if request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "gameId is required")
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
	state, err := h.gameService.MakeMove(ctx, gameplay.MoveCommand{GameID: request.GameID, ProfileID: client.profile.ID, Move: request.Move, Notation: moveNotation, ExpectedVersion: *request.ExpectedVersion})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.broadcastGameState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleResign(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.ResignPayload](message)
	if err != nil || request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid resign payload")
	}
	state, err := h.gameService.Resign(ctx, gameplay.ResignCommand{GameID: request.GameID, ProfileID: client.profile.ID})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.broadcastGameState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleOfferDraw(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.OfferDrawPayload](message)
	if err != nil || request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid draw.offer payload")
	}
	state, err := h.gameService.OfferDraw(ctx, gameplay.OfferDrawCommand{GameID: request.GameID, ProfileID: client.profile.ID})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.broadcastGameState(ctx, client, message.RequestID, state)
}

func (h *Handler) handleDrawResponse(ctx context.Context, client *Session, message protocol.ClientEnvelope, respond func(context.Context, gameplay.DrawOfferResponseCommand) (gameplay.GameSnapshot, error)) error {
	request, err := protocol.DecodePayload[protocol.DrawResponsePayload](message)
	if err != nil || request.GameID == "" || request.OfferID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid draw response payload")
	}
	state, err := respond(ctx, gameplay.DrawOfferResponseCommand{GameID: request.GameID, ProfileID: client.profile.ID, OfferID: request.OfferID})
	if err != nil {
		return h.sendApplicationError(ctx, client, message.RequestID, err)
	}
	return h.broadcastGameState(ctx, client, message.RequestID, state)
}

func (h *Handler) broadcastGameState(ctx context.Context, client *Session, requestID string, state gameplay.GameSnapshot) error {
	err := h.gameSessions.Broadcast(ctx, state.GameID, client, protocol.ServerEnvelope{Type: protocol.ServerGameState, RequestID: requestID, Payload: h.gameStatePayload(state)})
	if state.Outcome != chess.NoOutcome {
		h.gameSessions.RemoveGame(state.GameID)
	}
	return err
}
