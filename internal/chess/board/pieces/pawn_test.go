package pieces

import (
	"chessli/internal/chess/board/models"
	"testing"
)

func TestWhitePawnCanMoveOneOrTwoSquaresFromStartingRank(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank2, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)

	assertMoves(t, pawn.LegalMoves(from, board),
		models.NewPosition(models.Rank3, models.FileE),
		models.NewPosition(models.Rank4, models.FileE),
	)
}

func TestBlackPawnCanMoveOneOrTwoSquaresFromStartingRank(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank7, models.FileE)
	pawn := NewPawn(models.Black, from)
	board.place(pawn, from)

	assertMoves(t, pawn.LegalMoves(from, board),
		models.NewPosition(models.Rank6, models.FileE),
		models.NewPosition(models.Rank5, models.FileE),
	)
}

func TestPawnCannotMoveForwardWhenBlocked(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank2, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank3, models.FileE)), models.NewPosition(models.Rank3, models.FileE))

	assertMoves(t, pawn.LegalMoves(from, board))
}

func TestPawnCannotMoveTwoSquaresWhenSecondSquareIsBlocked(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank2, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank4, models.FileE)), models.NewPosition(models.Rank4, models.FileE))

	assertMoves(t, pawn.LegalMoves(from, board),
		models.NewPosition(models.Rank3, models.FileE),
	)
}

func TestPawnCanCaptureDiagonally(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileD)), models.NewPosition(models.Rank5, models.FileD))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))

	assertMoves(t, pawn.LegalMoves(from, board),
		models.NewPosition(models.Rank5, models.FileE),
		models.NewPosition(models.Rank5, models.FileD),
		models.NewPosition(models.Rank5, models.FileF),
	)
}

func TestPawnCannotCaptureForwardOrMoveDiagonallyToEmptySquare(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileE)), models.NewPosition(models.Rank5, models.FileE))

	assertMoves(t, pawn.LegalMoves(from, board))
}
