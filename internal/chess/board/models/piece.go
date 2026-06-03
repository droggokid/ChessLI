// Package models contains shared chess domain types used across board, pieces, and game.
package models

import (
	"fmt"
)

// PieceType identifies a concrete chess piece kind.
type PieceType string

const (
	Pawn   PieceType = "pawn"
	Knight PieceType = "knight"
	Bishop PieceType = "bishop"
	Rook   PieceType = "rook"
	Queen  PieceType = "queen"
	King   PieceType = "king"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=piece.go -destination=piece_mock.go -package=models

// Piece describes behavior shared by all concrete chess pieces.
type Piece interface {
	String() string
	Type() PieceType
	Color() Color
	Position() Position
	MoveTo(position Position)
	// PossibleMoves returns movement destinations before game-level king-safety filtering.
	PossibleMoves(board BoardView) []Position
	// AttackedSquares returns squares controlled by the piece.
	AttackedSquares(board BoardView) []Position
}

// BasePiece stores common piece state embedded by concrete piece implementations.
type BasePiece struct {
	PieceColor    Color    `json:"color"`
	PiecePosition Position `json:"position"`
}

// NewBasePiece creates common piece state for a piece with color and position.
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

// Describe formats a piece name with its color.
func (p *BasePiece) Describe(name string) string {
	return fmt.Sprintf("%s %s", p.PieceColor, name)
}
