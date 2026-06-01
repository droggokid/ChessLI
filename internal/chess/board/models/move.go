package models

import (
	"fmt"
)

// Move describes a requested move between two board positions.
type Move struct {
	From Position
	To   Position
}

// MoveRecord stores stable move history data for replay or persistence.
type MoveRecord struct {
	Move        Move
	MovingColor Color
	MovingPiece PieceType
	// CapturedPiece is empty when no piece was captured.
	CapturedPiece PieceType
	WasCapture    bool
}

// NewMove creates a move from one position to another.
func NewMove(from, to Position) Move {
	return Move{From: from, To: to}
}

func (m Move) String() string {
	return fmt.Sprintf("%s %s", m.From, m.To)
}
