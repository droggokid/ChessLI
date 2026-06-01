package pieces

import (
	"chessli/internal/chess/board/models"
)

// Pawn is a forward-moving piece with diagonal attacks.
type Pawn struct {
	models.BasePiece
}

// NewPawn creates a pawn with color and position.
func NewPawn(color models.Color, position models.Position) models.Piece {
	return &Pawn{
		BasePiece: models.NewBasePiece(color, position),
	}
}

func (p *Pawn) Type() models.PieceType {
	return models.Pawn
}

func (p *Pawn) String() string {
	if p == nil {
		return "pawn"
	}
	return p.Describe("pawn")
}

// PossibleMoves returns forward moves and diagonal captures.
func (p *Pawn) PossibleMoves(board models.BoardView) []models.Position {
	var (
		moveSet []models.Direction
		moves   []models.Position
	)

	if p.PieceColor == models.White {
		moveSet = whitePawnDirections
	} else {
		moveSet = blackPawnDirections
	}

	from := p.PiecePosition
	rank := from.Rank.ToIndex()
	file := from.File
	delta := moveSet[0].RankDelta

	shortMove := models.NewPosition(models.Rank(rank+delta), file)
	longMove := models.NewPosition(models.Rank(rank+(delta*2)), file)

	if emptySpot(board, shortMove) {
		moves = append(moves, shortMove)

		if p.isOnStartingRank(from) && emptySpot(board, longMove) {
			moves = append(moves, longMove)
		}
	}

	captureMoves := possibleMoves(from, moveSet[1:])

	for _, move := range captureMoves {
		if p.enemySpot(board, move) {
			moves = append(moves, move)
		}
	}

	return moves
}

func (p *Pawn) AttackedSquares(board models.BoardView) []models.Position {
	if p.PieceColor == models.White {
		return possibleMoves(p.PiecePosition, whitePawnDirections[1:])
	}

	return possibleMoves(p.PiecePosition, blackPawnDirections[1:])
}

func emptySpot(board models.BoardView, pos models.Position) bool {
	spot := board.SpotAt(pos)
	return spot != nil && spot.Piece == nil
}

func (p *Pawn) enemySpot(board models.BoardView, pos models.Position) bool {
	spot := board.SpotAt(pos)
	return spot != nil && spot.Piece != nil && spot.Piece.Color() != p.Color()
}

func (p *Pawn) isOnStartingRank(from models.Position) bool {
	if p.PieceColor == models.White {
		return from.Rank == models.Rank2
	}

	return from.Rank == models.Rank7
}
