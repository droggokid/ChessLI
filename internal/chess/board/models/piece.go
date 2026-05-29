// Package models contains shared chess domain types used across board, pieces, and game.
package models

import (
	"fmt"
)

// Piece describes behavior shared by all concrete chess pieces.
//
// PossibleMoves returns piece-shaped moves before game-level king-safety filtering.
// AttackedSquares returns controlled squares, which differs from possible movement
// for pieces such as pawns and for own-piece protection.
//
//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=piece.go -destination=mock_piece_test.go -package=models
type Piece interface {
	// String returns a human-readable piece description.
	String() string
	// Color returns the piece color.
	Color() Color
	// Position returns the piece's current board position.
	Position() Position
	// MoveTo updates the piece's stored position.
	MoveTo(position Position)
	// PossibleMoves returns movement destinations before game-level king-safety filtering.
	PossibleMoves(board BoardView) []Position
	// AttackedSquares returns squares controlled by the piece.
	AttackedSquares(board BoardView) []Position
}

// BasePiece stores common piece state embedded by concrete piece implementations.
type BasePiece struct {
	// PieceColor is the piece's side.
	PieceColor Color `json:"color"`
	// PiecePosition is the piece's current square.
	PiecePosition Position `json:"position"`
}

// NewBasePiece creates common piece state for a piece with color and position.
func NewBasePiece(color Color, position Position) BasePiece {
	return BasePiece{
		PieceColor:    color,
		PiecePosition: position,
	}
}

// Color returns the piece color.
func (p *BasePiece) Color() Color {
	return p.PieceColor
}

// Position returns the piece's current board position.
func (p *BasePiece) Position() Position {
	return p.PiecePosition
}

// MoveTo updates the piece's stored position.
func (p *BasePiece) MoveTo(position Position) {
	p.PiecePosition = position
}

// Describe formats a piece name with its color.
func (p *BasePiece) Describe(name string) string {
	return fmt.Sprintf("%s %s", p.PieceColor, name)
}
