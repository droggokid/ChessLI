package pieces

import (
	"chessli/internal/chess/board/models"
)

type Knight struct {
	models.BasePiece
}

func NewKnight(color models.Color, position models.Position) models.Piece {
	return &Knight{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (k *Knight) String() string {
	if k == nil {
		return "knight"
	}
	return k.Describe("knight")
}

func (k *Knight) LegalMoves(from models.Position, board models.BoardView) []models.Position {
	allMoves := possibleMoves(from, knightDirections)
	availableMoves := make([]models.Position, 0, len(allMoves))

	for _, move := range allMoves {
		spot := board.SpotAt(move)
		if canOccupy(spot, k) {
			availableMoves = append(availableMoves, move)
		}
	}

	return availableMoves
}
