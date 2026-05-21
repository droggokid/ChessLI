package pieces

import (
	"chessli/internal/chess/board/models"
)

type Pawn struct {
	models.BasePiece
}

func NewPawn(color models.Color, position models.Position) models.Piece {
	return &Pawn{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (p *Pawn) String() string {
	if p == nil {
		return "pawn"
	}
	return p.Describe("pawn")
}

func (p *Pawn) LegalMoves(from models.Position, board models.BoardView) []models.Position {
	return nil
}
