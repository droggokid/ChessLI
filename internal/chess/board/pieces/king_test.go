package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestKingPossibleMoves(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	king := NewKing(models.White, from)
	board.place(king, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank5, models.FileE)), models.NewPosition(models.Rank5, models.FileE))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank3, models.FileC)), models.NewPosition(models.Rank3, models.FileC))

	assertMoves(t, king.PossibleMoves(board, nil),
		models.NewPosition(models.Rank5, models.FileD),
		models.NewPosition(models.Rank3, models.FileD),
		models.NewPosition(models.Rank4, models.FileE),
		models.NewPosition(models.Rank4, models.FileC),
		models.NewPosition(models.Rank5, models.FileC),
		models.NewPosition(models.Rank3, models.FileE),
		models.NewPosition(models.Rank3, models.FileC),
	)
}

func TestKingAttackedSquares(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank1, models.FileA)
	king := NewKing(models.White, from)
	board.place(king, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank2, models.FileB)), models.NewPosition(models.Rank2, models.FileB))

	assertMoves(t, king.AttackedSquares(board),
		models.NewPosition(models.Rank2, models.FileA),
		models.NewPosition(models.Rank1, models.FileB),
		models.NewPosition(models.Rank2, models.FileB),
	)
}
