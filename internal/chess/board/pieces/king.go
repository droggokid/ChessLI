package pieces

import (
	"chessli/internal/chess/board/models"
)

type King struct {
	BasePiece models.Piece
}

func NewKing(color models.Color) models.Piece {
	return &King{
		BasePiece: models.NewPiece(color),
	}
}

func (king *King) String() string {
	if king == nil {
		return "king"
	}
	return models.DescribePiece("king", king.BasePiece)
}

func (king *King) LegalMoves() []models.Position {
	return nil
}

func (king *King) Move() {
	return
}
