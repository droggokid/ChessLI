package websocket

import (
	"errors"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
)

// Decoders preserve wire validation messages; gameplay validates application rules.
func decodeCreateGame(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.CreatePrivateCommand, error) {
	request, err := protocol.DecodePayload[protocol.CreateGamePayload](message)
	if err != nil {
		return gameplay.CreatePrivateCommand{}, errors.New("invalid game.create payload")
	}

	colorPreference, err := mapColorPreference(request.Color)
	if err != nil {
		return gameplay.CreatePrivateCommand{}, errors.New("invalid color preference")
	}

	initial, increment, err := mapTimeControlPreset(request.TimeControl)
	if err != nil {
		return gameplay.CreatePrivateCommand{}, errors.New("invalid time control preset")
	}

	return gameplay.CreatePrivateCommand{ProfileID: profileID, Initial: initial, Increment: increment, ColorPreference: colorPreference}, nil
}

func decodeJoinGame(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.JoinPrivateCommand, error) {
	request, err := protocol.DecodePayload[protocol.JoinGamePayload](message)
	if err != nil {
		return gameplay.JoinPrivateCommand{}, errors.New("invalid game.join payload")
	}
	if request.GameID == "" {
		return gameplay.JoinPrivateCommand{}, errors.New("gameId is required")
	}

	return gameplay.JoinPrivateCommand{ProfileID: profileID, GameID: request.GameID}, nil
}

func decodeResumeGame(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.ResumeGameCommand, error) {
	request, err := protocol.DecodePayload[protocol.ResumeGamePayload](message)
	if err != nil || request.GameID == "" {
		return gameplay.ResumeGameCommand{}, errors.New("invalid game.resume payload")
	}

	return gameplay.ResumeGameCommand{GameID: request.GameID, ProfileID: profileID}, nil
}

func decodeMove(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.MoveCommand, error) {
	request, err := protocol.DecodePayload[protocol.MovePayload](message)
	if err != nil {
		return gameplay.MoveCommand{}, errors.New("invalid game.move payload")
	}
	if request.GameID == "" {
		return gameplay.MoveCommand{}, errors.New("gameId is required")
	}
	if request.Move == "" {
		return gameplay.MoveCommand{}, errors.New("move is required")
	}
	if request.ExpectedVersion == nil {
		return gameplay.MoveCommand{}, errors.New("expectedVersion is required")
	}

	moveNotation, err := mapMoveNotation(request.Notation)
	if err != nil {
		return gameplay.MoveCommand{}, errors.New("unsupported move notation")
	}

	return gameplay.MoveCommand{GameID: request.GameID, ProfileID: profileID, Move: request.Move, Notation: moveNotation, ExpectedVersion: *request.ExpectedVersion}, nil
}

func decodeResign(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.ResignCommand, error) {
	request, err := protocol.DecodePayload[protocol.ResignPayload](message)
	if err != nil || request.GameID == "" {
		return gameplay.ResignCommand{}, errors.New("invalid resign payload")
	}

	return gameplay.ResignCommand{GameID: request.GameID, ProfileID: profileID}, nil
}

func decodeOfferDraw(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.OfferDrawCommand, error) {
	request, err := protocol.DecodePayload[protocol.OfferDrawPayload](message)
	if err != nil || request.GameID == "" {
		return gameplay.OfferDrawCommand{}, errors.New("invalid draw.offer payload")
	}

	return gameplay.OfferDrawCommand{GameID: request.GameID, ProfileID: profileID}, nil
}

func decodeDrawResponse(profileID identity.ProfileID, message protocol.ClientEnvelope) (gameplay.DrawOfferResponseCommand, error) {
	request, err := protocol.DecodePayload[protocol.DrawResponsePayload](message)
	if err != nil || request.GameID == "" || request.OfferID == "" {
		return gameplay.DrawOfferResponseCommand{}, errors.New("invalid draw response payload")
	}

	return gameplay.DrawOfferResponseCommand{GameID: request.GameID, ProfileID: profileID, OfferID: request.OfferID}, nil
}
