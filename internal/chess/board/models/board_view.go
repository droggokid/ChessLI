package models

// BoardView exposes read access to board spots for piece movement calculation.
//
//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=board_view.go -destination=mock_board_view_test.go -package=models
type BoardView interface {
	// SpotAt returns the board spot for pos, or nil when pos is outside the board.
	SpotAt(pos Position) *Spot
}
