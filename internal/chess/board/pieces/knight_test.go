package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestKnightPossibleMoves(t *testing.T) {
	tests := []struct {
		name  string
		from  models.Position
		place func(*testBoard)
		want  []models.Position
	}{
		{
			name: "all empty squares from center",
			from: models.NewPosition(models.Rank4, models.FileD),
			want: []models.Position{
				models.NewPosition(models.Rank6, models.FileE),
				models.NewPosition(models.Rank6, models.FileC),
				models.NewPosition(models.Rank5, models.FileF),
				models.NewPosition(models.Rank5, models.FileB),
				models.NewPosition(models.Rank2, models.FileE),
				models.NewPosition(models.Rank2, models.FileC),
				models.NewPosition(models.Rank3, models.FileF),
				models.NewPosition(models.Rank3, models.FileB),
			},
		},
		{
			name: "stays within board",
			from: models.NewPosition(models.Rank1, models.FileA),
			want: []models.Position{
				models.NewPosition(models.Rank3, models.FileB),
				models.NewPosition(models.Rank2, models.FileC),
			},
		},
		{
			name: "can capture enemy pieces",
			from: models.NewPosition(models.Rank4, models.FileD),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank6, models.FileE)), models.NewPosition(models.Rank6, models.FileE))
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))
			},
			want: []models.Position{
				models.NewPosition(models.Rank6, models.FileE),
				models.NewPosition(models.Rank6, models.FileC),
				models.NewPosition(models.Rank5, models.FileF),
				models.NewPosition(models.Rank5, models.FileB),
				models.NewPosition(models.Rank2, models.FileE),
				models.NewPosition(models.Rank2, models.FileC),
				models.NewPosition(models.Rank3, models.FileF),
				models.NewPosition(models.Rank3, models.FileB),
			},
		},
		{
			name: "cannot move to friendly occupied squares",
			from: models.NewPosition(models.Rank4, models.FileD),
			place: func(board *testBoard) {
				board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileE)), models.NewPosition(models.Rank6, models.FileE))
				board.place(NewPawn(models.White, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))
			},
			want: []models.Position{
				models.NewPosition(models.Rank6, models.FileC),
				models.NewPosition(models.Rank5, models.FileB),
				models.NewPosition(models.Rank2, models.FileE),
				models.NewPosition(models.Rank2, models.FileC),
				models.NewPosition(models.Rank3, models.FileF),
				models.NewPosition(models.Rank3, models.FileB),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			knight := NewKnight(models.White, tt.from)
			board.place(knight, tt.from)

			if tt.place != nil {
				tt.place(board)
			}

			assertMoves(t, knight.PossibleMoves(board), tt.want...)
		})
	}
}

func TestKnightAttackedSquares(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank1, models.FileA)
	knight := NewKnight(models.White, from)
	board.place(knight, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank3, models.FileB)), models.NewPosition(models.Rank3, models.FileB))

	assertMoves(t, knight.AttackedSquares(board),
		models.NewPosition(models.Rank3, models.FileB),
		models.NewPosition(models.Rank2, models.FileC),
	)
}
