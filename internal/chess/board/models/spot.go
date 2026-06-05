package models

import (
	"fmt"
)

// Spot represents one square on the board.
type Spot struct {
	Piece    Piece    `json:"piece,omitempty"`
	Position Position `json:"position"`
	Color    Color    `json:"color"`
}

// NewSpot creates a board square with an optional piece.
func NewSpot(piece Piece, position Position, color Color) *Spot {
	return &Spot{
		Piece:    piece,
		Position: position,
		Color:    color,
	}
}

func (s *Spot) String() string {
	if s == nil {
		return "<nil spot>"
	}

	piece := "empty"
	if s.Piece != nil {
		piece = s.Piece.String()
	}

	return fmt.Sprintf("%s %s", s.Position, piece)
}
