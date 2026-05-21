package pieces

import (
	"chessli/internal/chess/board/models"
)

type Queen struct {
	models.BasePiece
}

func NewQueen(color models.Color, position models.Position) models.Piece {
	return &Queen{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (q *Queen) String() string {
	if q == nil {
		return "queen"
	}
	return q.Describe("queen")
}

func (q *Queen) LegalMoves(from models.Position, board models.BoardView) []models.Position {
	return walkDirections(from, board, queenDirections)
}
