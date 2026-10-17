package websocket

import (
	"time"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

// Presence belongs to the transport registry, not the gameplay snapshot.
func mapGameState(state gameplay.GameSnapshot, whiteConnected, blackConnected bool) protocol.GameStatePayload {
	return protocol.GameStatePayload{
		GameID:           state.GameID,
		FEN:              state.FEN,
		Status:           mapGameStatus(state.Outcome),
		Version:          state.Version,
		White:            mapPlayerState(state.WhiteProfileID, protocol.ColorWhite, state.WhiteRemaining, whiteConnected),
		Black:            mapPlayerState(state.BlackProfileID, protocol.ColorBlack, state.BlackRemaining, blackConnected),
		LastMove:         state.LastMoveSAN,
		Outcome:          mapOutcome(state.Outcome, state.Termination),
		PendingDrawOffer: mapDrawOffer(state),
	}
}

func mapDrawOffer(state gameplay.GameSnapshot) *protocol.DrawOfferPayload {
	if state.PendingDrawOffer == nil {
		return nil
	}

	offeredBy := protocol.ColorBlack
	if state.PendingDrawOffer.OfferedBy == state.WhiteProfileID {
		offeredBy = protocol.ColorWhite
	}

	return &protocol.DrawOfferPayload{
		OfferID:   state.PendingDrawOffer.OfferID,
		OfferedBy: offeredBy,
	}
}

func mapPlayerState(profileID identity.ProfileID, color protocol.Color, timeRemaining time.Duration, connected bool) *protocol.PlayerState {
	if profileID == "" {
		return nil
	}

	return &protocol.PlayerState{
		ProfileID:             profileID,
		Color:                 color,
		Connected:             connected,
		RemainingMilliseconds: new(timeRemaining.Milliseconds()),
		Notation:              protocol.MoveNotationUCI,
	}
}

// Successful service assignments always contain White or Black.
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

func mapGameStatus(outcome game.Outcome) protocol.GameStatus {
	if outcome == game.OutcomeNone {
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

func mapOutcome(outcome game.Outcome, termination game.Termination) *protocol.GameOutcome {
	var result protocol.GameResult

	switch outcome {
	case game.OutcomeWhiteWin:
		result = protocol.ResultWhiteWin
	case game.OutcomeBlackWin:
		result = protocol.ResultBlackWin
	case game.OutcomeDraw:
		result = protocol.ResultDraw
	default:
		return nil
	}

	var reason protocol.GameOverReason

	switch termination {
	case game.TerminationCheckmate:
		reason = protocol.GameOverCheckmate
	case game.TerminationStalemate:
		reason = protocol.GameOverStalemate
	case game.TerminationResignation:
		reason = protocol.GameOverResignation
	case game.TerminationTimeout:
		reason = protocol.GameOverTimeout
	case game.TerminationDrawAgreement:
		reason = protocol.GameOverAgreement
	case game.TerminationThreefoldRepetition:
		reason = protocol.GameOverThreefoldRepetition
	case game.TerminationFivefoldRepetition:
		reason = protocol.GameOverFivefoldRepetition
	case game.TerminationFiftyMoveRule:
		reason = protocol.GameOverFiftyMoveRule
	case game.TerminationSeventyFiveMoveRule:
		reason = protocol.GameOverSeventyFiveMoveRule
	case game.TerminationInsufficientMaterial:
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
