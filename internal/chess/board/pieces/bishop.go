package pieces

import (
	"chessli/internal/chess/board/models"
)

type Bishop struct {
	models.BasePiece
}

func NewBishop(color models.Color, position models.Position) models.Piece {
	return &Bishop{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (b *Bishop) String() string {
	if b == nil {
		return "bishop"
	}
	return b.Describe("bishop")
}

func (b *Bishop) LegalMoves(from models.Position, board models.BoardView) []models.Position {
	return walkDirections(from, board, bishopDirections)
}
