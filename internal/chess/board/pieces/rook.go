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

// String returns a human-readable rook description.
func (r *Rook) String() string {
	if r == nil {
		return "rook"
	}
	return r.Describe("rook")
}

// PossibleMoves returns rook movement destinations before king-safety filtering.
func (r *Rook) PossibleMoves(board models.BoardView) []models.Position {
	return walkLegalDirections(r.PiecePosition, board, rookDirections)
}

// AttackedSquares returns all squares controlled by the rook.
func (r *Rook) AttackedSquares(board models.BoardView) []models.Position {
	return walkAttackDirections(r.PiecePosition, board, rookDirections)
}
