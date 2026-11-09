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

type JoinPrivateCommand struct {
	ProfileID identity.ProfileID
	GameID    identity.GameID
}

type ResumeGameCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
}

type EnterMatchmakingCommand struct {
	ProfileID identity.ProfileID
	Initial   time.Duration
	Increment time.Duration
}

type MoveCommand struct {
	GameID          identity.GameID
	ProfileID       identity.ProfileID
	Move            string
	Notation        MoveNotation
	ExpectedVersion uint64
}

type ResignCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
}

type OfferDrawCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
}

type DrawOfferResponseCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
	OfferID   identity.DrawOfferID
}
