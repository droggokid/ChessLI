package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
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

	if _, exists := h.gameSessions.GameID(client); exists {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"session is already in a game",
		)
	}

	colorPreference, err := mapColorPreference(request.Color)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid color preference")
	}

	command := gameplay.CreatePrivateCommand{
		ProfileID:       client.profileID,
		ColorPreference: colorPreference,
	}

	if request.TimeControl != nil {
		command.Initial = time.Duration(request.TimeControl.InitialMilliseconds) * time.Millisecond

		command.Increment = time.Duration(request.TimeControl.IncrementMilliseconds) * time.Millisecond
	}

	result, err := h.gameService.CreatePrivateGame(ctx, command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	if err = h.gameSessions.Add(result.GameID, client); err != nil {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"session is already in a game",
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
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid game.join payload")
	}

	if request.GameID == "" {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "gameId is required")
	}

	if _, exists := h.gameSessions.GameID(client); exists {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"session is already in a game",
		)
	}

	command := gameplay.NewJoinPrivateCommand(client.profileID, request.GameID)

	serviceResult, err := h.gameService.JoinPrivateGame(ctx, *command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	if err = h.gameSessions.Add(serviceResult.GameID, client); err != nil {
		return h.sendError(
			ctx,
			client,
			message.RequestID,
			protocol.ErrorInvalidMessage,
			"session is already in a game",
		)
	}

	return client.Send(ctx, protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload: protocol.GameJoinedPayload{
			GameID: serviceResult.GameID,
			Color:  mapColorFromServer(serviceResult.Color),
		},
	})
}

func (h *Handler) handleMatchmaking(ctx context.Context, client *Session, message protocol.ClientEnvelope) error {
	return h.sendNotImplemented(ctx, client, message)
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

	serviceResult, err := h.gameService.MakeMove(ctx, *command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	gameStatus := mapGameStatus(serviceResult.Outcome)

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerGameState,
		RequestID: message.RequestID,
		Payload: protocol.GameStatePayload{
			GameID:   serviceResult.GameID,
			FEN:      serviceResult.FEN,
			Status:   gameStatus,
			Version:  serviceResult.Version,
			White:    nil,
			Black:    nil,
			LastMove: serviceResult.SAN,
		},
	}

	if err = h.gameSessions.Broadcast(ctx, serviceResult.GameID, client, envelope); err != nil {
		slog.Warn("broadcast game state", "game_id", serviceResult.GameID, "error", err)
	}

	return nil
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

	case errors.Is(err, gameplay.ErrGameNotReady):
		return protocol.ErrorIllegalMove, "game is waiting for another player"

	case errors.Is(err, gameplay.ErrInvalidColorPreference):
		return protocol.ErrorInvalidMessage, "invalid color preference"

	case errors.Is(err, gameplay.ErrInvalidTimeControl):
		return protocol.ErrorInvalidMessage, "invalid time control"

	case errors.Is(err, gameplay.ErrUnsupportedNotation):
		return protocol.ErrorInvalidMessage, "unsupported move notation"

	case errors.Is(err, gameplay.ErrAlreadyParticipant):
		return protocol.ErrorInvalidMessage, "already a participant in this game"

	case errors.Is(err, gameplay.ErrAlreadyQueued):
		return protocol.ErrorInvalidMessage, "already queued for matchmaking"

	case errors.Is(err, gameplay.ErrNoCompatibleOpponent):
		return protocol.ErrorInvalidMessage, "no compatible opponent available"

	case errors.Is(err, gameplay.ErrStaleGameVersion):
		return protocol.ErrorStaleGameVersion, "game state is stale"

	default:
		return protocol.ErrorInternal, "internal server error"
	}
}

func mapColorFromServer(color chess.Color) protocol.Color {
	if color == chess.White {
		return protocol.ColorWhite
	}
	return protocol.ColorBlack
}

func mapMoveNotation(notation protocol.MoveNotation) (gameplay.MoveNotation, error) {
	switch notation {
	case "", protocol.MoveNotationUCI:
		return gameplay.MoveNotationUCI, nil

	case protocol.MoveNotationSAN:
		return gameplay.MoveNotationSAN, nil

	case protocol.MoveNotationLAN:
		return gameplay.MoveNotationLAN, nil

	default:
		return 0, gameplay.ErrUnsupportedNotation
	}
}

func mapGameStatus(outcome chess.Outcome) protocol.GameStatus {
	if outcome == chess.NoOutcome {
		return protocol.GameStatusActive
	}

	return protocol.GameStatusFinished
}

func mapColorPreference(preference protocol.ColorPreference) (gameplay.ColorPreference, error) {
	switch preference {
	case "", protocol.ColorPreferenceRandom:
		return gameplay.ColorRandom, nil

	case protocol.ColorPreferenceWhite:
		return gameplay.ColorWhite, nil

	case protocol.ColorPreferenceBlack:
		return gameplay.ColorBlack, nil

	default:
		return 0, gameplay.ErrInvalidColorPreference
	}
}
