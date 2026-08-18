package protocol

type GameStatus string

const (
	GameStatusWaiting  GameStatus = "waiting"
	GameStatusActive   GameStatus = "active"
	GameStatusFinished GameStatus = "finished"
	GameStatusAborted  GameStatus = "aborted"
)

type Color string

const (
	ColorWhite Color = "white"
	ColorBlack Color = "black"
)

type PlayerState struct {
	ID                    string       `json:"id"`
	Color                 Color        `json:"color"`
	Connected             bool         `json:"connected"`
	RemainingMilliseconds *int64       `json:"remainingMilliseconds,omitempty"`
	Notation              MoveNotation `json:"notation"`
}

type ColorPreference string

const (
	ColorPreferenceWhite  ColorPreference = "white"
	ColorPreferenceBlack  ColorPreference = "black"
	ColorPreferenceRandom ColorPreference = "random"
)

type GameResult string

const (
	ResultWhiteWin GameResult = "white_win"
	ResultBlackWin GameResult = "black_win"
	ResultDraw     GameResult = "draw"
)

type GameOverReason string

const (
	GameOverCheckmate            GameOverReason = "checkmate"
	GameOverStalemate            GameOverReason = "stalemate"
	GameOverResignation          GameOverReason = "resignation"
	GameOverTimeout              GameOverReason = "timeout"
	GameOverAgreement            GameOverReason = "draw_agreement"
	GameOverThreefoldRepetition  GameOverReason = "threefold_repetition"
	GameOverFiftyMoveRule        GameOverReason = "fifty_move_rule"
	GameOverInsufficientMaterial GameOverReason = "insufficient_material"
)

type GameOutcome struct {
	Result GameResult     `json:"result"`
	Reason GameOverReason `json:"reason"`
}
