package board

import (
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"strings"
)

type Board struct {
	Spots [8][8]*models.Spot `json:"spots"`
}

func NewBoard() *Board {
	board := &Board{Spots: [8][8]*models.Spot{}}

	board.arrangeBoard()
	board.arrangePieces()

	return board
}
func (b *Board) String() string {
	return b.stringByRankOrder(models.Rank8, models.Rank1, -1)
}

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
		b.Spots[models.Rank2][file].Piece = pieces.NewPawn(models.White)
	}
	for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
		b.Spots[models.Rank7][file].Piece = pieces.NewPawn(models.Black)
	}
}

func (b *Board) placeBackRank(files []models.File, newPiece func(models.Color) models.Piece) {
	for _, f := range files {
		i := f.ToIndex()
		b.Spots[models.Rank1][i].Piece = newPiece(models.White)
		b.Spots[models.Rank8][i].Piece = newPiece(models.Black)
	}
}
