package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

type Handler struct {
	gameService gameplay.Service
}

// NewHandler returns a WebSocket message handler backed by gameService.
func NewHandler(gameService gameplay.Service) *Handler {
	return &Handler{gameService: gameService}
}

// Handle decodes and dispatches a client message.
func (h *Handler) Handle(ctx context.Context, client *Session, raw json.RawMessage) error {
	var message protocol.ClientEnvelope

	if err := json.Unmarshal(raw, &message); err != nil {
		return h.sendError(
			ctx,
			client,
			"",
			protocol.ErrorInvalidMessage,
			"message must be valid JSON",
		)
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
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorUnknownType,
			fmt.Sprintf("unknown message type %q", message.Type),
		)
	}
}

func (h *Handler) handleCreateGame(
	ctx context.Context,
	client *Session,
	message protocol.ClientEnvelope,
) error {
	request, err := protocol.DecodePayload[protocol.CreateGamePayload](message)
	if err != nil {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"invalid game.create payload",
		)
	}

	command := gameplay.CreatePrivateCommand{
		ProfileID: client.profileID,
	}

	if request.TimeControl != nil {
		command.Initial = time.Duration(request.TimeControl.InitialMilliseconds)

		command.Increment = time.Duration(request.TimeControl.IncrementMilliseconds)
	}

	result, err := h.gameService.CreatePrivateGame(ctx, command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(
			ctx,
			client,
			message.RequestID,
			code,
			publicMessage,
		)
	}

	color := protocol.ColorWhite
	if result.Color == chess.Black {
		color = protocol.ColorBlack
	}

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameCreated,
		RequestID: message.RequestID,
		Payload: protocol.GameCreatedPayload{
			GameID: result.GameID,
			Color:  color,
		},
	})
}

func (h *Handler) handleJoinGame(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.JoinGamePayload](message)
	if err != nil {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"invalid game.join payload",
		)
	}

	if request.GameID == "" {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"gameId is required",
		)
	}

	// Later:
	// result, err := h.games.Join(ctx, client.ID(), request.GameID)

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload: protocol.GameJoinedPayload{
			GameID: request.GameID,
			Color:  protocol.ColorBlack,
		},
	})
}

func (h *Handler) handleMatchmaking(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
}

func (h *Handler) handleMakeMove(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	request, err := protocol.DecodePayload[protocol.MovePayload](message)
	if err != nil {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"invalid game.move payload",
		)
	}

	if request.Move == "" {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"move is required",
		)
	}

	// handle move

	return h.sendNotImplemented(ctx, client, message)
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

func (h *Handler) sendError(
	ctx context.Context,
	client *Session,
	requestID string,
	code protocol.ErrorCode,
	message string,
) error {
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
	return h.sendError(
		ctx,
		client,
		message.RequestID,
		protocol.ErrorNotImplemented,
		fmt.Sprintf("%s is not implemented", message.Type),
	)
}

func mapApplicationError(err error) (protocol.ErrorCode, string) {
	switch {
	case errors.Is(err, gameplay.ErrGameNotFound):
		return protocol.ErrorGameNotFound, "game not found"

	case errors.Is(err, gameplay.ErrGameFull):
		return protocol.ErrorGameFull, "game is full"

	case errors.Is(err, gameplay.ErrNotParticipant):
		return protocol.ErrorNotPlayer, "not a player in this game"

	case errors.Is(err, gameplay.ErrNotYourTurn):
		return protocol.ErrorNotYourTurn, "not your turn"

	case errors.Is(err, gameplay.ErrIllegalMove):
		return protocol.ErrorIllegalMove, "illegal move"

	case errors.Is(err, gameplay.ErrGameFinished):
		return protocol.ErrorGameFinished, "game is finished"

	default:
		return protocol.ErrorInternal, "internal server error"
	}
}
