package gameplay

import "errors"

var (
	ErrGameNotFound           = errors.New("game not found")
	ErrGameFull               = errors.New("game is full")
	ErrNotParticipant         = errors.New("profile is not a participant in this game")
	ErrNotYourTurn            = errors.New("not your turn")
	ErrIllegalMove            = errors.New("illegal move")
	ErrGameFinished           = errors.New("game is finished")
	ErrInvalidColorPreference = errors.New("invalid color preference")
	ErrAlreadyParticipant     = errors.New("already participant")
	ErrAlreadyQueued          = errors.New("already queued")
	ErrGameNotReady           = errors.New("game is waiting for another player")
	ErrInvalidTimeControl     = errors.New("invalid time control")
	ErrUnsupportedNotation    = errors.New("unsupported move notation")
	ErrStaleGameVersion       = errors.New("stale game version")
)
