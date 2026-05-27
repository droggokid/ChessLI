package pieces

import (
	"chessli/internal/chess/board/models"
	"testing"
)

type testBoard struct {
	spots [8][8]*models.Spot
}

func newTestBoard() *testBoard {
	board := &testBoard{}
	for rank := models.Rank1; rank <= models.Rank8; rank++ {
		for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
			position := models.NewPosition(rank, models.ToFile(file))
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

func assertMoves(t *testing.T, got []models.Position, want ...models.Position) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d moves %v, want %d moves %v", len(got), got, len(want), want)
	}

	gotMoves := make(map[models.Position]bool, len(got))
	for _, move := range got {
		gotMoves[move] = true
	}

	for _, move := range want {
		if !gotMoves[move] {
			t.Fatalf("missing move %v; got %v", move, got)
		}
	}
}
