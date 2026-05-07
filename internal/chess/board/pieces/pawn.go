package pieces

import (
	"chessli/internal/chess/board/models"
)

type Pawn struct {
	BasePiece models.Piece
}

func NewPawn(color models.Color) models.Piece {
	return &Pawn{
		BasePiece: models.NewPiece(color),
	}
}

func (pawn *Pawn) String() string {
	if pawn == nil {
		return "pawn"
	}
	return models.DescribePiece("pawn", pawn.BasePiece)
}

func (pawn *Pawn) LegalMoves() []models.Position {
	return nil
}

func (pawn *Pawn) Move() {
	return
}
