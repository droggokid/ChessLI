package game

import (
	"testing"

	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
)

func TestMoveUpdatesKingPosition(t *testing.T) {
	game := newEmptyGame(models.White)
	from := models.NewPosition(models.Rank1, models.FileE)
	to := models.NewPosition(models.Rank2, models.FileE)
	move := NewMove(from, to)

	placePiece(game, pieces.NewKing(models.White, from), from)
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
	refreshGameState(t, game)

	if err := game.Move(move); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if game.WhiteKingPosition != to {
		t.Fatalf("WhiteKingPosition = %v, want %v", game.WhiteKingPosition, to)
	}
	if game.Board.SpotAt(from).Piece != nil {
		t.Fatalf("from spot still has piece: %v", game.Board.SpotAt(from).Piece)
	}
	if game.Board.SpotAt(to).Piece == nil {
		t.Fatal("to spot has no piece")
	}
}

func TestMoveRecordsHistory(t *testing.T) {
	game := newEmptyGame(models.White)
	from := models.NewPosition(models.Rank1, models.FileA)
	to := models.NewPosition(models.Rank8, models.FileA)
	move := NewMove(from, to)

	placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileE)), models.NewPosition(models.Rank1, models.FileE))
	placePiece(game, pieces.NewRook(models.White, from), from)
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileH)), models.NewPosition(models.Rank8, models.FileH))
	placePiece(game, pieces.NewKnight(models.Black, to), to)
	refreshGameState(t, game)

	if err := game.Move(move); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if len(game.MoveHistory) != 1 {
		t.Fatalf("len(MoveHistory) = %d, want 1", len(game.MoveHistory))
	}

	record := game.MoveHistory[0]
	if record.Move != move {
		t.Fatalf("MoveHistory[0].Move = %v, want %v", record.Move, move)
	}
	if record.MovingColor != models.White {
		t.Fatalf("MovingColor = %v, want %v", record.MovingColor, models.White)
	}
	if record.MovingPiece != models.Rook {
		t.Fatalf("MovingPiece = %v, want %v", record.MovingPiece, models.Rook)
	}
	if record.CapturedPiece != models.Knight {
		t.Fatalf("CapturedPiece = %v, want %v", record.CapturedPiece, models.Knight)
	}
	if !record.WasCapture {
		t.Fatal("WasCapture = false, want true")
	}
}
