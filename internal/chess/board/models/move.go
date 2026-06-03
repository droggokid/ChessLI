package models

import "fmt"

// Move represents a requested board transition.
type Move struct {
	From Position
	To   Position
}

type ResolvedMove struct {
	Move          Move
	FromSpot      *Spot
	ToSpot        *Spot
	MovingPiece   Piece
	CapturedPiece Piece
}

// NewMove creates a move from one position to another.
func NewMove(from, to Position) Move {
	return Move{From: from, To: to}
}

func (m Move) String() string {
	return fmt.Sprintf("%s %s", m.From, m.To)
}
