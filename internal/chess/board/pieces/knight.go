package pieces

import (
	"chessli/internal/chess/board/models"
)

type Knight struct {
	BasePiece models.Piece
}

func NewKnight(color models.Color) models.Piece {
	return &Knight{
		BasePiece: models.NewPiece(color),
	}
}

func (knight *Knight) String() string {
	if knight == nil {
		return "knight"
	}
	return models.DescribePiece("knight", knight.BasePiece)
}

func (knight *Knight) LegalMoves() []models.Position {
	return nil
}

func (knight *Knight) Move() {
	return
}
