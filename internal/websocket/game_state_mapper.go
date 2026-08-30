package websocket

import (
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

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
		White:    h.playerState(state.GameID, state.WhiteProfileID, protocol.ColorWhite, state.WhiteRemaining),
		Black:    h.playerState(state.GameID, state.BlackProfileID, protocol.ColorBlack, state.BlackRemaining),
		LastMove: state.LastMoveSAN,
		Outcome:  mapOutcome(state.Outcome, state.Termination),
	}
}

func (h *Handler) playerState(gameID identity.GameID, profileID identity.ProfileID, color protocol.Color, timeRemaining time.Duration) *protocol.PlayerState {
	if profileID == "" {
		return nil
	}

	return &protocol.PlayerState{
		ProfileID:             profileID,
		Color:                 color,
		Connected:             h.gameSessions.IsConnected(gameID, profileID),
		RemainingMilliseconds: new(timeRemaining.Milliseconds()),
		Notation:              protocol.MoveNotationUCI,
	}
}

func mapColorFromServer(color chess.Color) (protocol.Color, error) {
	switch color {
	case chess.White:
		return protocol.ColorWhite, nil
	case chess.Black:
		return protocol.ColorBlack, nil
	default:
		return "", gameplay.ErrInvalidColorPreference
	}
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

func mapOutcome(outcome chess.Outcome, termination gameplay.TerminationReason) *protocol.GameOutcome {
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

	switch termination {
	case gameplay.TerminationCheckmate:
		reason = protocol.GameOverCheckmate
	case gameplay.TerminationStalemate:
		reason = protocol.GameOverStalemate
	case gameplay.TerminationResignation:
		reason = protocol.GameOverResignation
	case gameplay.TerminationTimeout:
		reason = protocol.GameOverTimeout
	case gameplay.TerminationDrawAgreement:
		reason = protocol.GameOverAgreement
	case gameplay.TerminationThreefoldRepetition:
		reason = protocol.GameOverThreefoldRepetition
	case gameplay.TerminationFivefoldRepetition:
		reason = protocol.GameOverFivefoldRepetition
	case gameplay.TerminationFiftyMoveRule:
		reason = protocol.GameOverFiftyMoveRule
	case gameplay.TerminationSeventyFiveMoveRule:
		reason = protocol.GameOverSeventyFiveMoveRule
	case gameplay.TerminationInsufficientMaterial:
		reason = protocol.GameOverInsufficientMaterial
	default:
		return nil
	}

	return &protocol.GameOutcome{Result: result, Reason: reason}
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
