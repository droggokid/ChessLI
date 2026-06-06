package pieces

import (
	"chessli/internal/chess/board/models"
)

// Knight is a leaping piece that moves in L-shaped offsets.
type Knight struct {
	models.BasePiece
}

// NewKnight creates a knight with color and position.
func NewKnight(color models.Color, position models.Position) models.Piece {
	return &Knight{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (k *Knight) Type() models.PieceType {
	return models.Knight
}

func (k *Knight) String() string {
	if k == nil {
		return "knight"
	}
	return k.Describe("knight")
}

func (k *Knight) PossibleMoves(board models.BoardView, _ *models.Move) []models.Position {
	allMoves := possibleMoves(k.PiecePosition, knightDirections)
	availableMoves := make([]models.Position, 0, len(allMoves))

	for _, move := range allMoves {
		spot := board.SpotAt(move)
		if canOccupy(spot, k) {
			availableMoves = append(availableMoves, move)
		}
	}

	return availableMoves
}

func (k *Knight) AttackedSquares(board models.BoardView) []models.Position {
	return possibleMoves(k.PiecePosition, knightDirections)
}
