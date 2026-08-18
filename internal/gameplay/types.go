package gameplay

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
