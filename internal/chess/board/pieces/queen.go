package pieces

import (
	"chessli/internal/chess/board/models"
)

// Queen is a sliding piece that combines rook and bishop movement.
type Queen struct {
	models.BasePiece
}

// NewQueen creates a queen with color and position.
func NewQueen(color models.Color, position models.Position) models.Piece {
	return &Queen{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (q *Queen) Type() models.PieceType {
	return models.Queen
}

func (q *Queen) String() string {
	if q == nil {
		return "queen"
	}
	return q.Describe("queen")
}

func (q *Queen) PossibleMoves(board models.BoardView, _ *models.Move) []models.Position {
	return walkLegalDirections(q.PiecePosition, board, queenDirections)
}

func (q *Queen) AttackedSquares(board models.BoardView) []models.Position {
	return walkAttackDirections(q.PiecePosition, board, queenDirections)
}
