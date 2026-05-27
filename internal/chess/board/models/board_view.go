package models

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=board_view.go -destination=mock_board_view_test.go -package=models
type BoardView interface {
	SpotAt(pos Position) *Spot
}
