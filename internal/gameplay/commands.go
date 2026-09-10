package gameplay

import (
	"time"

	"ChessLI/internal/identity"
)

type ColorPreference uint8

const (
	ColorRandom ColorPreference = iota
	ColorWhite
	ColorBlack
)

type CreatePrivateCommand struct {
	ProfileID       identity.ProfileID
	Initial         time.Duration
	Increment       time.Duration
	ColorPreference ColorPreference
}

// NewCreatePrivateCommand builds a command for creating a private game.
func NewCreatePrivateCommand(profileID identity.ProfileID, initial time.Duration, increment time.Duration, colorPreference ColorPreference) CreatePrivateCommand {
	return CreatePrivateCommand{
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

// NewJoinPrivateCommand builds a command for joining a private game.
func NewJoinPrivateCommand(profileID identity.ProfileID, gameID identity.GameID) JoinPrivateCommand {
	return JoinPrivateCommand{
		ProfileID: profileID,
		GameID:    gameID,
	}
}

type EnterMatchmakingCommand struct {
	ProfileID   identity.ProfileID
	timeControl timeControlKey
}

// NewEnterMatchmakingCommand builds a command for entering a time-control pool.
func NewEnterMatchmakingCommand(profileID identity.ProfileID, initial time.Duration, increment time.Duration) EnterMatchmakingCommand {
	return EnterMatchmakingCommand{
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

// NewMoveCommand builds a versioned move command for a game participant.
func NewMoveCommand(gameID identity.GameID, profileID identity.ProfileID, move string, notation MoveNotation, expectedVersion uint64) MoveCommand {
	return MoveCommand{
		GameID:          gameID,
		ProfileID:       profileID,
		Move:            move,
		Notation:        notation,
		ExpectedVersion: expectedVersion,
	}
}

type ResignCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
}

// NewResignCommand builds a resign command for a game participant.
func NewResignCommand(gameID identity.GameID, profileID identity.ProfileID) ResignCommand {
	return ResignCommand{
		GameID:    gameID,
		ProfileID: profileID,
	}
}

type OfferDrawCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
}

// NewOfferDrawCommand builds a draw-offer command for a game participant.
func NewOfferDrawCommand(gameID identity.GameID, profileID identity.ProfileID) OfferDrawCommand {
	return OfferDrawCommand{
		GameID:    gameID,
		ProfileID: profileID,
	}
}

type DrawOfferResponseCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
	OfferID   identity.DrawOfferID
}

// NewDrawOfferResponseCommand builds a command responding to a draw offer.
func NewDrawOfferResponseCommand(gameID identity.GameID, profileID identity.ProfileID, offerID identity.DrawOfferID) DrawOfferResponseCommand {
	return DrawOfferResponseCommand{
		GameID:    gameID,
		ProfileID: profileID,
		OfferID:   offerID,
	}
}
