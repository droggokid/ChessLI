package pieces

import (
	"chessli/internal/chess/board/models"
)

type Bishop struct {
	BasePiece models.Piece
}

func NewBishop(color models.Color) models.Piece {
	return &Bishop{
		BasePiece: models.NewPiece(color),
	}
}

func (bishop *Bishop) String() string {
	if bishop == nil {
		return "bishop"
	}
	return models.DescribePiece("bishop", bishop.BasePiece)
}

func (bishop *Bishop) LegalMoves() []models.Position {
	return nil
}

func (bishop *Bishop) Move() {
	return
}
