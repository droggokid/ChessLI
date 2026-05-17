package pieces

import (
	"chessli/internal/chess/board/models"
)

type Rook struct {
	models.BasePiece
}

func NewRook(color models.Color, position models.Position) models.Piece {
	return &Rook{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (r *Rook) String() string {
	if r == nil {
		return "rook"
	}
	return r.Describe("rook")
}

func (r *Rook) LegalMoves() []models.Position {
	return nil
}
