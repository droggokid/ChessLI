package pieces

import (
	"chessli/internal/chess/board/models"
)

type Queen struct {
	BasePiece models.Piece
}

func NewQueen(color models.Color) models.Piece {
	return &Queen{
		BasePiece: models.NewPiece(color),
	}
}

func (queen *Queen) String() string {
	if queen == nil {
		return "queen"
	}
	return models.DescribePiece("queen", queen.BasePiece)
}

func (queen *Queen) LegalMoves() []models.Position {
	return nil
}

func (queen *Queen) Move() {
	return
}
