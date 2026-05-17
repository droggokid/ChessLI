package pieces

import (
	"chessli/internal/chess/board/models"
)

type King struct {
	models.BasePiece
}

func NewKing(color models.Color, position models.Position) models.Piece {
	return &King{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (k *King) String() string {
	if k == nil {
		return "king"
	}
	return k.Describe("king")
}

func (k *King) LegalMoves() []models.Position {
	return nil
}
