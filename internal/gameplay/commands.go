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

type JoinPrivateCommand struct {
	ProfileID identity.ProfileID
	GameID    identity.GameID
}

type EnterMatchmakingCommand struct {
	ProfileID identity.ProfileID
	Initial   time.Duration
	Increment time.Duration
}

type MoveCommand struct {
	GameID    identity.GameID
	ProfileID identity.ProfileID
	Move      string
}
