package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestPawnPossibleMoves(t *testing.T) {
	tests := []struct {
		name  string
		color models.Color
		from  models.Position
		place func(*testBoard)
		want  []models.Position
	}{
		{
			name:  "white can move one or two squares from starting rank",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank3, models.FileE),
				models.NewPosition(models.Rank4, models.FileE),
			},
		},
		{
			name:  "black can move one or two squares from starting rank",
			color: models.Black,
			from:  models.NewPosition(models.Rank7, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank6, models.FileE),
				models.NewPosition(models.Rank5, models.FileE),
			},
		},
		{
			name:  "cannot move forward when blocked",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank3, models.FileE)), models.NewPosition(models.Rank3, models.FileE))
			},
		},
		{
			name:  "cannot move two squares when second square is blocked",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank4, models.FileE)), models.NewPosition(models.Rank4, models.FileE))
			},
			want: []models.Position{
				models.NewPosition(models.Rank3, models.FileE),
			},
		},
		{
			name:  "can capture diagonally",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileD)), models.NewPosition(models.Rank5, models.FileD))
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))
			},
			want: []models.Position{
				models.NewPosition(models.Rank5, models.FileE),
				models.NewPosition(models.Rank5, models.FileD),
				models.NewPosition(models.Rank5, models.FileF),
			},
		},
		{
			name:  "cannot capture forward or move diagonally to empty square",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileE)), models.NewPosition(models.Rank5, models.FileE))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			pawn := NewPawn(tt.color, tt.from)
			board.place(pawn, tt.from)

			if tt.place != nil {
				tt.place(board)
			}

			assertMoves(t, pawn.PossibleMoves(board), tt.want...)
		})
	}
}

func TestPawnAttackedSquares(t *testing.T) {
	tests := []struct {
		name  string
		color models.Color
		from  models.Position
		want  []models.Position
	}{
		{
			name:  "white attacks diagonally north",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank5, models.FileD),
				models.NewPosition(models.Rank5, models.FileF),
			},
		},
		{
			name:  "black attacks diagonally south",
			color: models.Black,
			from:  models.NewPosition(models.Rank5, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank4, models.FileD),
				models.NewPosition(models.Rank4, models.FileF),
			},
		},
		{
			name:  "edge pawn only attacks in-bounds squares",
			color: models.White,
			from:  models.NewPosition(models.Rank7, models.FileA),
			want: []models.Position{
				models.NewPosition(models.Rank8, models.FileB),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			pawn := NewPawn(tt.color, tt.from)
			board.place(pawn, tt.from)

			assertMoves(t, pawn.AttackedSquares(board), tt.want...)
		})
	}
}
