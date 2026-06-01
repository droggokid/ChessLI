// Package pieces contains concrete chess piece implementations and movement helpers.
package pieces

import (
	"chessli/internal/chess/board/models"
)

// Bishop is a diagonal sliding piece.
type Bishop struct {
	models.BasePiece
}

// NewBishop creates a bishop with color and position.
func NewBishop(color models.Color, position models.Position) models.Piece {
	return &Bishop{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (b *Bishop) Type() models.PieceType {
	return models.Bishop
}

func (b *Bishop) String() string {
	if b == nil {
		return "bishop"
	}
	return b.Describe("bishop")
}

func (b *Bishop) PossibleMoves(board models.BoardView) []models.Position {
	return walkLegalDirections(b.PiecePosition, board, bishopDirections)
}

func (b *Bishop) AttackedSquares(board models.BoardView) []models.Position {
	return walkAttackDirections(b.PiecePosition, board, bishopDirections)
}
