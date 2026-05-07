package pieces

import (
	"chessli/internal/chess/board/models"
)

type Rook struct {
	BasePiece models.Piece
}

func NewRook(color models.Color) models.Piece {
	return &Rook{
		BasePiece: models.NewPiece(color),
	}
}

func (rook *Rook) String() string {
	if rook == nil {
		return "rook"
	}
	return models.DescribePiece("rook", rook.BasePiece)
}

func (rook *Rook) LegalMoves() []models.Position {
	return nil
}

func (rook *Rook) Move() {
	return
}
