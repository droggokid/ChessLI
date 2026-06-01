package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestBishopPossibleMoves(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	bishop := NewBishop(models.White, from)
	board.place(bishop, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileF)), models.NewPosition(models.Rank6, models.FileF))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank6, models.FileB)), models.NewPosition(models.Rank6, models.FileB))

	assertMoves(t, bishop.PossibleMoves(board),
		models.NewPosition(models.Rank5, models.FileE),
		models.NewPosition(models.Rank5, models.FileC),
		models.NewPosition(models.Rank6, models.FileB),
		models.NewPosition(models.Rank3, models.FileE),
		models.NewPosition(models.Rank2, models.FileF),
		models.NewPosition(models.Rank1, models.FileG),
		models.NewPosition(models.Rank3, models.FileC),
		models.NewPosition(models.Rank2, models.FileB),
		models.NewPosition(models.Rank1, models.FileA),
	)
}

func TestBishopAttackedSquaresIncludesFriendlyBlocker(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	bishop := NewBishop(models.White, from)
	board.place(bishop, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileF)), models.NewPosition(models.Rank6, models.FileF))

	assertMoves(t, bishop.AttackedSquares(board),
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
