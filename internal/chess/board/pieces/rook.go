package pieces

import (
	"chessli/internal/chess/board/models"
)

// Rook is an orthogonal sliding piece.
type Rook struct {
	models.BasePiece
}

// NewRook creates a rook with color and position.
func NewRook(color models.Color, position models.Position) models.Piece {
	return &Rook{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (r *Rook) Type() models.PieceType {
	return models.Rook
}

func (r *Rook) String() string {
	if r == nil {
		return "rook"
	}
	return r.Describe("rook")
}

func (r *Rook) PossibleMoves(board models.BoardView) []models.Position {
	return walkLegalDirections(r.PiecePosition, board, rookDirections)
}

func (r *Rook) AttackedSquares(board models.BoardView) []models.Position {
	return walkAttackDirections(r.PiecePosition, board, rookDirections)
}
