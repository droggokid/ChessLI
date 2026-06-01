package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestQueenPossibleMoves(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	queen := NewQueen(models.White, from)
	board.place(queen, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileD)), models.NewPosition(models.Rank6, models.FileD))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank6, models.FileF)), models.NewPosition(models.Rank6, models.FileF))

	assertMoves(t, queen.PossibleMoves(board),
		models.NewPosition(models.Rank5, models.FileD),
		models.NewPosition(models.Rank3, models.FileD),
		models.NewPosition(models.Rank2, models.FileD),
		models.NewPosition(models.Rank1, models.FileD),
		models.NewPosition(models.Rank4, models.FileE),
		models.NewPosition(models.Rank4, models.FileF),
		models.NewPosition(models.Rank4, models.FileG),
		models.NewPosition(models.Rank4, models.FileH),
		models.NewPosition(models.Rank4, models.FileC),
		models.NewPosition(models.Rank4, models.FileB),
		models.NewPosition(models.Rank4, models.FileA),
		models.NewPosition(models.Rank5, models.FileE),
		models.NewPosition(models.Rank6, models.FileF),
		models.NewPosition(models.Rank5, models.FileC),
		models.NewPosition(models.Rank6, models.FileB),
		models.NewPosition(models.Rank7, models.FileA),
		models.NewPosition(models.Rank3, models.FileE),
		models.NewPosition(models.Rank2, models.FileF),
		models.NewPosition(models.Rank1, models.FileG),
		models.NewPosition(models.Rank3, models.FileC),
		models.NewPosition(models.Rank2, models.FileB),
		models.NewPosition(models.Rank1, models.FileA),
	)
}

func TestQueenAttackedSquaresIncludesFriendlyBlocker(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	queen := NewQueen(models.White, from)
	board.place(queen, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileD)), models.NewPosition(models.Rank6, models.FileD))

	assertMoves(t, queen.AttackedSquares(board),
		models.NewPosition(models.Rank5, models.FileD),
		models.NewPosition(models.Rank6, models.FileD),
		models.NewPosition(models.Rank3, models.FileD),
		models.NewPosition(models.Rank2, models.FileD),
		models.NewPosition(models.Rank1, models.FileD),
		models.NewPosition(models.Rank4, models.FileE),
		models.NewPosition(models.Rank4, models.FileF),
		models.NewPosition(models.Rank4, models.FileG),
		models.NewPosition(models.Rank4, models.FileH),
		models.NewPosition(models.Rank4, models.FileC),
		models.NewPosition(models.Rank4, models.FileB),
		models.NewPosition(models.Rank4, models.FileA),
		models.NewPosition(models.Rank5, models.FileE),
		models.NewPosition(models.Rank6, models.FileF),
		models.NewPosition(models.Rank7, models.FileG),
		models.NewPosition(models.Rank8, models.FileH),
		models.NewPosition(models.Rank5, models.FileC),
		models.NewPosition(models.Rank6, models.FileB),
		models.NewPosition(models.Rank7, models.FileA),
		models.NewPosition(models.Rank3, models.FileE),
		models.NewPosition(models.Rank2, models.FileF),
		models.NewPosition(models.Rank1, models.FileG),
		models.NewPosition(models.Rank3, models.FileC),
		models.NewPosition(models.Rank2, models.FileB),
		models.NewPosition(models.Rank1, models.FileA),
	)
}
