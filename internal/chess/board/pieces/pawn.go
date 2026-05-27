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
	var (
		moveSet []models.Direction
		moves   []models.Position
	)

	if p.PieceColor == models.White {
		moveSet = whitePawnDirections
	} else {
		moveSet = blackPawnDirections
	}

	rank := from.Rank.ToIndex()
	file := from.File
	delta := moveSet[0].RankDelta

	shortMove := models.NewPosition(models.Rank(rank+delta), file)
	longMove := models.NewPosition(models.Rank(rank+(delta*2)), file)
	possibleMoves := possibleMoves(from, moveSet)

	if emptySpot(board, shortMove) {
		moves = append(moves, shortMove)

		if p.isOnStartingRank(from) && emptySpot(board, longMove) {
			moves = append(moves, longMove)
		}
	}

	if enemySpot(board, possibleMoves[1], p) {
		moves = append(moves, possibleMoves[1])
	}

	if enemySpot(board, possibleMoves[2], p) {
		moves = append(moves, possibleMoves[2])
	}

	return moves
}

func emptySpot(board models.BoardView, pos models.Position) bool {
	spot := board.SpotAt(pos)
	return spot != nil && spot.Piece == nil
}

func enemySpot(board models.BoardView, pos models.Position, movingPiece models.Piece) bool {
	spot := board.SpotAt(pos)
	return spot != nil && spot.Piece != nil && spot.Piece.Color() != movingPiece.Color()
}

func (p *Pawn) isOnStartingRank(from models.Position) bool {
	if p.PieceColor == models.White {
		return from.Rank == models.Rank2
	}

	return from.Rank == models.Rank7
}

func possibleMoves(from models.Position, directions []models.Direction) []models.Position {
	moves := make([]models.Position, 0)
	rank := from.Rank.ToIndex()
	file := from.File.ToIndex()
	for _, dir := range directions {
		pos := models.NewPosition(models.Rank(rank+dir.RankDelta), models.ToFile(file+dir.FileDelta))
		moves = append(moves, pos)
	}

	return moves
}
