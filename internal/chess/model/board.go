package model

import "strings"

type Board struct {
	Spots [8][8]Spot `json:"spots"`
}

func NewBoard() *Board {
	board := &Board{Spots: [8][8]Spot{}}

	board.arrangeBoard()
	board.arrangePieces()

	return board
}
func (b *Board) String() string {
	return b.stringByRankOrder(Rank8, Rank1, -1)
}

func (b *Board) StringBlackView() string {
	return b.stringByRankOrder(Rank1, Rank8, 1)
}

func (b *Board) stringByRankOrder(start Rank, end Rank, step int) string {
	if b == nil {
		return "<nil board>"
	}

	var builder strings.Builder
	first := true
	for rank := start.ToIndex(); ; rank += step {
		for file := FileA.ToIndex(); file <= FileH.ToIndex(); file++ {
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
	color := Black
	for rank := Rank1; rank <= Rank8; rank++ {
		b.createFileWithFirstSpotColor(color, rank)
		color = color.Flip()
	}
}

func (b *Board) createFileWithFirstSpotColor(color Color, rank Rank) {
	for file := FileA.ToIndex(); file <= FileH.ToIndex(); file++ {
		b.Spots[rank.ToIndex()][file] = NewSpot(nil, NewPosition(rank, ToFile(file)), color)
		color = color.Flip()
	}
}

func (b *Board) arrangePieces() {
	b.placeBackRank([]File{FileA, FileH}, NewRook)
	b.placeBackRank([]File{FileB, FileG}, NewKnight)
	b.placeBackRank([]File{FileC, FileF}, NewBishop)
	b.placeBackRank([]File{FileD}, NewQueen)
	b.placeBackRank([]File{FileE}, NewKing)

	b.createPawns()
}

func (b *Board) createPawns() {
	for file := FileA.ToIndex(); file <= FileH.ToIndex(); file++ {
		b.Spots[Rank2][file].Piece = NewPawn(White)
	}
	for file := FileA.ToIndex(); file <= FileH.ToIndex(); file++ {
		b.Spots[Rank7][file].Piece = NewPawn(Black)
	}
}

func (b *Board) placeBackRank(files []File, newPiece func(Color) Piece) {
	for _, f := range files {
		i := f.ToIndex()
		b.Spots[Rank1][i].Piece = newPiece(White)
		b.Spots[Rank8][i].Piece = newPiece(Black)
	}
}
