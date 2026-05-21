package models

type BoardView interface {
	SpotAt(pos Position) *Spot
}
