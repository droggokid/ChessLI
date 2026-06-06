package pieces

import (
	"chessli/internal/chess/board/models"
)

// King is the royal piece whose safety determines legal move filtering.
type King struct {
	models.BasePiece
}

// NewKing creates a king with color and position.
func NewKing(color models.Color, position models.Position) models.Piece {
	return &King{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (k *King) Type() models.PieceType {
	return models.King
}

func (k *King) String() string {
	if k == nil {
		return "king"
	}
	return k.Describe("king")
}

// PossibleMoves returns adjacent destinations before attacked-square filtering.
func (k *King) PossibleMoves(board models.BoardView, _ *models.Move) []models.Position {
	moves := make([]models.Position, 0)
	for _, pos := range possibleMoves(k.PiecePosition, kingDirections) {
		spot := board.SpotAt(pos)
		if canOccupy(spot, k) {
			moves = append(moves, pos)
		}
	}

	return moves
}

func (k *King) AttackedSquares(board models.BoardView) []models.Position {
	return possibleMoves(k.PiecePosition, kingDirections)
}
