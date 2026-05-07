package models

import (
	"fmt"
)

type Piece interface {
	String() string
	Move()
	LegalMoves() []Position
}

type BasePiece struct {
	Color Color `json:"color"`
}

func NewPiece(color Color) Piece {
	return &BasePiece{
		Color: color,
	}
}

func (p *BasePiece) Move() {
	//TODO implement me
	panic("implement me")
}

func (p *BasePiece) String() string {
	if p == nil {
		return "unknown"
	}
	return p.Color.String()
}

func (p *BasePiece) LegalMoves() []Position {
	return nil
}

func DescribePiece(name string, base Piece) string {
	if base == nil {
		return name
	}
	return fmt.Sprintf("%s %s", base.String(), name)
}
