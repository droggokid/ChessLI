package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

type testBoard struct {
	spots [8][8]*models.Spot
}

func newTestBoard() *testBoard {
	board := &testBoard{}
	for rank := models.Rank1; rank <= models.Rank8; rank++ {
		for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
			position := pos(rank, models.ToFile(file))
			board.spots[rank.ToIndex()][file] = models.NewSpot(nil, position, models.White)
		}
	}
	return board
}

func (b *testBoard) SpotAt(pos models.Position) *models.Spot {
	if !pos.IsValid() {
		return nil
	}
	return b.spots[pos.Rank.ToIndex()][pos.File.ToIndex()]
}

func (b *testBoard) place(piece models.Piece, pos models.Position) {
	b.SpotAt(pos).Piece = piece
}

func (b *testBoard) placePawn(color models.Color, rank models.Rank, file models.File) {
	position := pos(rank, file)
	b.place(NewPawn(color, position), position)
}

func pos(rank models.Rank, file models.File) models.Position {
	return models.NewPosition(rank, file)
}

func positionPointer(position models.Position) *models.Position {
	return &position
}

func assertMoves(t *testing.T, got []models.Position, want ...models.Position) {
	t.Helper()

	gotMoves := positionSet(got)
	wantMoves := positionSet(want)

	if len(gotMoves) != len(wantMoves) {
		t.Fatalf("moves = %v, want %v", got, want)
	}

	for move := range wantMoves {
		if !gotMoves[move] {
			t.Fatalf("moves = %v, missing %v", got, move)
		}
	}

	for move := range gotMoves {
		if !wantMoves[move] {
			t.Fatalf("moves = %v, unexpected %v", got, move)
		}
	}
}

func positionSet(positions []models.Position) map[models.Position]bool {
	set := make(map[models.Position]bool, len(positions))
	for _, position := range positions {
		set[position] = true
	}
	return set
}
