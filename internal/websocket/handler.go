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

	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid time control preset")
	}

	command := gameplay.CreatePrivateCommand{
		ProfileID:       client.profileID,
		ColorPreference: colorPreference,
		Initial:         initial,
		Increment:       increment,
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

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerGameJoined,
		RequestID: message.RequestID,
		Payload: protocol.GameJoinedPayload{
			GameID: serviceResult.GameID,
			Color:  mapColorFromServer(serviceResult.Color),
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

	if _, exists := h.gameSessions.GameID(client); exists {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "session is already in a game")
	}

	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return h.sendError(ctx, client, message.RequestID, protocol.ErrorInvalidMessage, "invalid time control preset")
	}

	command := gameplay.NewEnterMatchmakingCommand(client.profileID, initial, increment)

	ticket, err := h.gameService.EnterMatchmaking(ctx, *command)
	if err != nil {
		code, publicMessage := mapApplicationError(err)

		return h.sendError(ctx, client, message.RequestID, code, publicMessage)
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerMatchmakingEntered,
		RequestID: message.RequestID,
		Payload:   protocol.EnterMatchmakingPayload{TimeControl: request.TimeControl},
	}

	if err = client.Send(ctx, envelope); err != nil {
		return err
	}

	go h.awaitMatch(ctx, client, message.RequestID, ticket)

	return nil
}

func (h *Handler) awaitMatch(ctx context.Context, client *Session, requestID string, ticket gameplay.MatchTicket) {
	var result gameplay.MatchResult

	select {
	case <-ctx.Done():
		return
	case match, ok := <-ticket.Result:
		if !ok {
			return
		}

		result = match
	}

	envelope := protocol.ServerEnvelope{
		Type:      protocol.ServerMatchFound,
		RequestID: requestID,
		Payload: protocol.MatchFoundPayload{
			GameID: result.GameID,
			Color:  mapColorFromServer(result.Color),
		},
	}
	if err := client.Send(ctx, envelope); err != nil {
		slog.Warn("send matchmaking result", "error", err)
		return
	}

	if err := h.gameSessions.Add(result.GameID, client); err != nil {
		slog.Warn("register matched session", "game_id", result.GameID, "error", err)
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

	if err := client.Send(ctx, h.initialState(state)); err != nil {
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

func (h *Handler) initialState(state gameplay.GameSnapshot) protocol.ServerEnvelope {
	return protocol.ServerEnvelope{
		Type:    protocol.ServerGameInitial,
		Payload: h.gameStatePayload(state),
	}
}

func (h *Handler) gameStatePayload(state gameplay.GameSnapshot) protocol.GameStatePayload {
	return protocol.GameStatePayload{
		GameID:   state.GameID,
		FEN:      state.FEN,
		Status:   mapGameStatus(state.Outcome),
		Version:  state.Version,
		White:    nil,
		Black:    nil,
		LastMove: state.LastMoveSAN,
		Outcome:  mapOutcome(state.Outcome, state.Method),
	}
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

func mapOutcome(outcome chess.Outcome, method chess.Method) *protocol.GameOutcome {
	if outcome == chess.NoOutcome || outcome == chess.UnknownOutcome {
		return nil
	}

	var result protocol.GameResult

	switch outcome {
	case chess.WhiteWon:
		result = protocol.ResultWhiteWin
	case chess.BlackWon:
		result = protocol.ResultBlackWin
	case chess.Draw:
		result = protocol.ResultDraw
	default:
		return nil
	}

	var reason protocol.GameOverReason

	switch method {
	case chess.Checkmate:
		reason = protocol.GameOverCheckmate
	case chess.Resignation:
		reason = protocol.GameOverResignation
	case chess.DrawOffer:
		reason = protocol.GameOverAgreement
	case chess.Stalemate:
		reason = protocol.GameOverStalemate
	case chess.ThreefoldRepetition, chess.FivefoldRepetition:
		reason = protocol.GameOverThreefoldRepetition
	case chess.FiftyMoveRule, chess.SeventyFiveMoveRule:
		reason = protocol.GameOverFiftyMoveRule
	case chess.InsufficientMaterial:
		reason = protocol.GameOverInsufficientMaterial
	default:
		return nil
	}

	return &protocol.GameOutcome{
		Result: result,
		Reason: reason,
	}
}

func mapTimeControlPreset(preset protocol.TimeControlPreset) (time.Duration, time.Duration, error) {
	switch preset {
	case protocol.TimeControlBullet1Plus0:
		return time.Minute, 0, nil
	case protocol.TimeControlBullet1Plus1:
		return time.Minute, time.Second, nil
	case protocol.TimeControlBullet2Plus1:
		return 2 * time.Minute, time.Second, nil
	case protocol.TimeControlBlitz3Plus0:
		return 3 * time.Minute, 0, nil
	case protocol.TimeControlBlitz3Plus2:
		return 3 * time.Minute, 2 * time.Second, nil
	case protocol.TimeControlBlitz5Plus0:
		return 5 * time.Minute, 0, nil
	case protocol.TimeControlRapid10Plus0:
		return 10 * time.Minute, 0, nil
	case protocol.TimeControlRapid10Plus5:
		return 10 * time.Minute, 5 * time.Second, nil
	case protocol.TimeControlRapid15Plus10:
		return 15 * time.Minute, 10 * time.Second, nil
	case protocol.TimeControlClassical30Plus0:
		return 30 * time.Minute, 0, nil
	default:
		return 0, 0, gameplay.ErrInvalidTimeControl
	}
}
