package gameplay

import (
	"time"

	"ChessLI/internal/identity"
)

type CreatePrivateCommand struct {
	ProfileID       identity.ProfileID
	Initial         time.Duration
	Increment       time.Duration
	ColorPreference ColorPreference
}

func NewCreatePrivateCommand(profileID identity.ProfileID, initial time.Duration, increment time.Duration, colorPreference ColorPreference) *CreatePrivateCommand {
	return &CreatePrivateCommand{
		ProfileID:       profileID,
		Initial:         initial,
		Increment:       increment,
		ColorPreference: colorPreference,
	}
}

type JoinPrivateCommand struct {
	ProfileID identity.ProfileID
	GameID    identity.GameID
}

func NewJoinPrivateCommand(profileID identity.ProfileID, gameID identity.GameID) *JoinPrivateCommand {
	return &JoinPrivateCommand{
		ProfileID: profileID,
		GameID:    gameID,
	}
}

type EnterMatchmakingCommand struct {
	ProfileID   identity.ProfileID
	timeControl timeControlKey
}

func NewEnterMatchmakingCommand(profileID identity.ProfileID, initial time.Duration, increment time.Duration) *EnterMatchmakingCommand {
	return &EnterMatchmakingCommand{
		ProfileID: profileID,
		timeControl: timeControlKey{
			initial:   initial,
			increment: increment,
		},
	}
}

type MoveCommand struct {
	GameID          identity.GameID
	ProfileID       identity.ProfileID
	Move            string
	Notation        MoveNotation
	ExpectedVersion uint64
}

func NewMoveCommand(gameID identity.GameID, profileID identity.ProfileID, move string, notation MoveNotation, expectedVersion uint64) *MoveCommand {
	return &MoveCommand{
		GameID:          gameID,
		ProfileID:       profileID,
		Move:            move,
		Notation:        notation,
		ExpectedVersion: expectedVersion,
	}
}
