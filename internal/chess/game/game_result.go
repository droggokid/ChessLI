package game

type GameResult struct {
	Winner *Player
	Draw   bool
}

func NewGameResult() *GameResult {
	return &GameResult{Winner: nil, Draw: false}
}
