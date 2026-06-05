package game

type GameState string

const (
	GameStateActive    GameState = "active"
	GameStateCheck     GameState = "check"
	GameStateCheckmate GameState = "checkmate"
	GameStateDraw      GameState = "draw"
)
