// Package board owns board construction, square lookup, and initial piece setup.
package board

import (
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"strings"
)

// Board represents an 8x8 chess board.
type Board struct {
	Spots [8][8]*models.Spot `json:"spots"`
}

// NewBoard creates a standard chess board with colored squares and starting pieces.
func NewBoard() *Board {
	board := &Board{Spots: [8][8]*models.Spot{}}
	board.arrangeBoard()
	board.arrangePieces()
	return board
}

// SpotAt returns the board spot for pos, or nil when pos is outside the board.
func (b *Board) SpotAt(pos models.Position) *models.Spot {
	if !pos.IsValid() {
		return nil
	}
	return b.Spots[pos.Rank.ToIndex()][pos.File.ToIndex()]
}

func (b *Board) String() string {
	return b.stringByRankOrder(models.Rank8, models.Rank1, -1)
}

// StringBlackView returns a black-side board view from rank 1 up to rank 8.
func (b *Board) StringBlackView() string {
	return b.stringByRankOrder(models.Rank1, models.Rank8, 1)
}

func (b *Board) stringByRankOrder(start models.Rank, end models.Rank, step int) string {
	if b == nil {
		return "<nil board>"
	}

	var builder strings.Builder
	first := true
	for rank := start.ToIndex(); ; rank += step {
		for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
			if !first {
				builder.WriteByte('\n')
			}
			first = false
			builder.WriteString(b.Spots[rank][file].String())
		}
		if rank == end.ToIndex() {
			break
		}
	}
	return builder.String()
}

func (b *Board) arrangeBoard() {
	color := models.Black
	for rank := models.Rank1; rank <= models.Rank8; rank++ {
		b.createFileWithFirstSpotColor(color, rank)
		color = color.Flip()
	}
}

func (b *Board) createFileWithFirstSpotColor(color models.Color, rank models.Rank) {
	for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
		b.Spots[rank.ToIndex()][file] = models.NewSpot(nil, models.NewPosition(rank, models.ToFile(file)), color)
		color = color.Flip()
	}
}

func (b *Board) arrangePieces() {
	b.placeBackRank([]models.File{models.FileA, models.FileH}, pieces.NewRook)
	b.placeBackRank([]models.File{models.FileB, models.FileG}, pieces.NewKnight)
	b.placeBackRank([]models.File{models.FileC, models.FileF}, pieces.NewBishop)
	b.placeBackRank([]models.File{models.FileD}, pieces.NewQueen)
	b.placeBackRank([]models.File{models.FileE}, pieces.NewKing)

	b.createPawns()
}

func (b *Board) createPawns() {
	for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
		whiteBoardSpot := b.Spots[models.Rank2][file]

		whiteBoardSpot.Piece = pieces.NewPawn(models.White, whiteBoardSpot.Position)
	}
	for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
		blackBoardSpot := b.Spots[models.Rank7][file]

		blackBoardSpot.Piece = pieces.NewPawn(models.Black, blackBoardSpot.Position)
	}
}

func (b *Board) placeBackRank(files []models.File, newPiece func(models.Color, models.Position) models.Piece) {
	for _, f := range files {
		i := f.ToIndex()
		whiteBoardSpot := b.Spots[models.Rank1][i]
		blackBoardSpot := b.Spots[models.Rank8][i]

		whiteBoardSpot.Piece = newPiece(models.White, whiteBoardSpot.Position)
		blackBoardSpot.Piece = newPiece(models.Black, blackBoardSpot.Position)
	}
}

// WhiteStarterPieces returns the white pieces from their initial board squares.
func (b *Board) WhiteStarterPieces() []models.Piece {
	whitePieces := make([]models.Piece, 0, 16)

	for _, spot := range b.Spots[models.Rank1.ToIndex()] {
		whitePieces = append(whitePieces, spot.Piece)
	}
	for _, spot := range b.Spots[models.Rank2.ToIndex()] {
		whitePieces = append(whitePieces, spot.Piece)
	}

	return whitePieces
}

// BlackStarterPieces returns the black pieces from their initial board squares.
func (b *Board) BlackStarterPieces() []models.Piece {
	blackPieces := make([]models.Piece, 0, 16)

	for _, spot := range b.Spots[models.Rank8.ToIndex()] {
		blackPieces = append(blackPieces, spot.Piece)
	}
	for _, spot := range b.Spots[models.Rank7.ToIndex()] {
		blackPieces = append(blackPieces, spot.Piece)
	}

	return blackPieces
}
