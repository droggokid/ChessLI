package models

import (
	"fmt"
)

// Spot represents one square on the board.
type Spot struct {
	// Piece is the piece occupying the square, or nil when empty.
	Piece Piece `json:"piece,omitempty"`
	// Position is the square coordinate.
	Position Position `json:"position"`
	// Color is the visual board-square color.
	Color Color `json:"color"`
}

// NewSpot creates a board square with an optional piece.
func NewSpot(piece Piece, position Position, color Color) *Spot {
	return &Spot{
		Piece:    piece,
		Position: position,
		Color:    color,
	}
}

// String returns a human-readable spot description.
func (s *Spot) String() string {
	if s == nil {
		return "<nil spot>"
	}

	piece := "empty"
	if s.Piece != nil {
		piece = s.Piece.String()
	}

	return fmt.Sprintf("%s [%s] %s", s.Position, s.Color, piece)
}
