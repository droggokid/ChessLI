package models

import (
	"fmt"
)

type Piece interface {
	String() string
	Color() Color
	Position() Position
	MoveTo(position Position)
	LegalMoves(from Position, board BoardView) []Position
}

type BasePiece struct {
	PieceColor    Color    `json:"color"`
	PiecePosition Position `json:"position"`
}

func NewBasePiece(color Color, position Position) BasePiece {
	return BasePiece{
		PieceColor:    color,
		PiecePosition: position,
	}
}

func (p *BasePiece) Color() Color {
	return p.PieceColor
}

func (p *BasePiece) Position() Position {
	return p.PiecePosition
}

func (p *BasePiece) MoveTo(position Position) {
	p.PiecePosition = position
}

func (p *BasePiece) Describe(name string) string {
	return fmt.Sprintf("%s %s", p.PieceColor, name)
}
