package pieces

import (
	"chessli/internal/chess/board/models"
)

func canOccupy(spot *models.Spot, movingPiece models.Piece) bool {
	if spot == nil {
		return false
	}

	if spot.Piece == nil {
		return true
	}

	return spot.Piece.Color() != movingPiece.Color()
}

func walkDirection(from models.Position, board models.BoardView, offset models.Direction) []models.Position {
	var moves []models.Position

	fromSpot := board.SpotAt(from)
	if fromSpot == nil || fromSpot.Piece == nil {
		return moves
	}

	movingPiece := fromSpot.Piece

	rank := from.Rank.ToIndex() + offset.RankDelta
	file := from.File.ToIndex() + offset.FileDelta

	for rank >= 0 && rank < 8 && file >= 0 && file < 8 {
		pos := models.NewPosition(models.Rank(rank), models.ToFile(file))
		spot := board.SpotAt(pos)

		if spot == nil {
			break
		}

		if canOccupy(spot, movingPiece) {
			moves = append(moves, pos)
		}

		if spot.Piece != nil {
			break
		}

		rank += offset.RankDelta
		file += offset.FileDelta
	}
	return moves
}

func walkDirections(from models.Position, board models.BoardView, directions []models.Direction) []models.Position {
	var moves []models.Position

	for _, direction := range directions {
		moves = append(moves, walkDirection(from, board, direction)...)
	}

	return moves
}

func possibleMoves(from models.Position, directions []models.Direction) []models.Position {
	moves := make([]models.Position, 0)
	rank := from.Rank.ToIndex()
	file := from.File.ToIndex()
	for _, dir := range directions {
		pos := models.NewPosition(models.Rank(rank+dir.RankDelta), models.ToFile(file+dir.FileDelta))
		if pos.IsValid() {
			moves = append(moves, pos)
		}
	}

	return moves
}

var queenDirections = []models.Direction{
	models.North,
	models.South,
	models.East,
	models.West,
	models.NorthEast,
	models.NorthWest,
	models.SouthEast,
	models.SouthWest,
}

var kingDirections = []models.Direction{
	models.North,
	models.South,
	models.East,
	models.West,
	models.NorthEast,
	models.NorthWest,
	models.SouthEast,
	models.SouthWest,
}

var knightDirections = []models.Direction{
	{RankDelta: 2, FileDelta: 1},
	{RankDelta: 2, FileDelta: -1},
	{RankDelta: 1, FileDelta: 2},
	{RankDelta: 1, FileDelta: -2},
	{RankDelta: -2, FileDelta: 1},
	{RankDelta: -2, FileDelta: -1},
	{RankDelta: -1, FileDelta: 2},
	{RankDelta: -1, FileDelta: -2},
}

var bishopDirections = []models.Direction{
	models.NorthEast,
	models.NorthWest,
	models.SouthEast,
	models.SouthWest,
}

var rookDirections = []models.Direction{
	models.North,
	models.South,
	models.East,
	models.West,
}

var whitePawnDirections = []models.Direction{
	models.North,
	models.NorthEast,
	models.NorthWest,
}

var blackPawnDirections = []models.Direction{
	models.South,
	models.SouthEast,
	models.SouthWest,
}
