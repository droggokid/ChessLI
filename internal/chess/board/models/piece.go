package models

import (
	"fmt"
)

type Piece interface {
	String() string
	LegalMoves() []Position
}

type BasePiece struct {
	Color    Color    `json:"color"`
	Position Position `json:"position"`
}

func NewBasePiece(color Color, position Position) BasePiece {
	return BasePiece{
		Color:    color,
		Position: position,
	}
}

func (p BasePiece) Describe(name string) string {
	return fmt.Sprintf("%s %s", p.Color, name)
}
