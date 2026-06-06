package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestRookPossibleMoves(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	rook := NewRook(models.White, from)
	board.place(rook, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileD)), models.NewPosition(models.Rank6, models.FileD))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank4, models.FileB)), models.NewPosition(models.Rank4, models.FileB))

	assertMoves(t, rook.PossibleMoves(board, nil),
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
	)
}

func TestRookAttackedSquaresIncludesFriendlyBlocker(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	rook := NewRook(models.White, from)
	board.place(rook, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileD)), models.NewPosition(models.Rank6, models.FileD))

	assertMoves(t, rook.AttackedSquares(board),
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
	)
}
