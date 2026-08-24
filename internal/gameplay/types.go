package gameplay

import "time"

type GameMode uint8

const (
	GameModePrivate GameMode = iota
	GameModeMatchmaking
)

type ColorPreference uint8

const (
	ColorRandom ColorPreference = iota
	ColorWhite
	ColorBlack
)

type waitingPlayer struct {
	command EnterMatchmakingCommand
	result  chan MatchResult
	done    <-chan struct{}
}

type MoveNotation uint8

const (
	MoveNotationUCI MoveNotation = iota
	MoveNotationSAN
	MoveNotationLAN
)

// timeControlKey identifies one matchmaking pool by its clock settings.
type timeControlKey struct {
	initial   time.Duration
	increment time.Duration
}
