package pieces

import (
	"chessli/internal/chess/board/models"
	"testing"
)

func TestKnightCanMoveToAllEmptySquaresFromCenter(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	knight := NewKnight(models.White, from)
	board.place(knight, from)

	assertMoves(t, knight.LegalMoves(from, board),
		models.NewPosition(models.Rank6, models.FileE),
		models.NewPosition(models.Rank6, models.FileC),
		models.NewPosition(models.Rank5, models.FileF),
		models.NewPosition(models.Rank5, models.FileB),
		models.NewPosition(models.Rank2, models.FileE),
		models.NewPosition(models.Rank2, models.FileC),
		models.NewPosition(models.Rank3, models.FileF),
		models.NewPosition(models.Rank3, models.FileB),
	)
}

func TestKnightMovesStayWithinBoard(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank1, models.FileA)
	knight := NewKnight(models.White, from)
	board.place(knight, from)

	assertMoves(t, knight.LegalMoves(from, board),
		models.NewPosition(models.Rank3, models.FileB),
		models.NewPosition(models.Rank2, models.FileC),
	)
}

func TestKnightCanCaptureEnemyPieces(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	knight := NewKnight(models.White, from)
	board.place(knight, from)
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank6, models.FileE)), models.NewPosition(models.Rank6, models.FileE))
	board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))

	assertMoves(t, knight.LegalMoves(from, board),
		models.NewPosition(models.Rank6, models.FileE),
		models.NewPosition(models.Rank6, models.FileC),
		models.NewPosition(models.Rank5, models.FileF),
		models.NewPosition(models.Rank5, models.FileB),
		models.NewPosition(models.Rank2, models.FileE),
		models.NewPosition(models.Rank2, models.FileC),
		models.NewPosition(models.Rank3, models.FileF),
		models.NewPosition(models.Rank3, models.FileB),
	)
}

func TestKnightCannotMoveToFriendlyOccupiedSquares(t *testing.T) {
	board := newTestBoard()
	from := models.NewPosition(models.Rank4, models.FileD)
	knight := NewKnight(models.White, from)
	board.place(knight, from)
	board.place(NewPawn(models.White, models.NewPosition(models.Rank6, models.FileE)), models.NewPosition(models.Rank6, models.FileE))
	board.place(NewPawn(models.White, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))

	assertMoves(t, knight.LegalMoves(from, board),
		models.NewPosition(models.Rank6, models.FileC),
		models.NewPosition(models.Rank5, models.FileB),
		models.NewPosition(models.Rank2, models.FileE),
		models.NewPosition(models.Rank2, models.FileC),
		models.NewPosition(models.Rank3, models.FileF),
		models.NewPosition(models.Rank3, models.FileB),
	)
}
