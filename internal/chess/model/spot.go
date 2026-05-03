package model

import (
	"fmt"
)

type Spot struct {
	Piece    Piece    `json:"piece,omitempty"`
	Position Position `json:"position"`
	Color    Color    `json:"color"`
}

func NewSpot(piece Piece, position Position, color Color) Spot {
	return Spot{
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

	return fmt.Sprintf("%s [%s] %s", s.Position, s.Color, piece)
}
