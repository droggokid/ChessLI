package rules

import (
	"testing"

	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"chessli/internal/chess/moves"
)

func TestLegalMovesForFiltersPinnedPieceMove(t *testing.T) {
	gameBoard := newEmptyBoard(t)
	whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
	blackKingPos := models.NewPosition(models.Rank8, models.FileA)
	pinnedRookPos := models.NewPosition(models.Rank2, models.FileE)
	checkingRookPos := models.NewPosition(models.Rank8, models.FileE)

	whiteKing := pieces.NewKing(models.White, whiteKingPos)
	blackKing := pieces.NewKing(models.Black, blackKingPos)
	pinnedRook := pieces.NewRook(models.White, pinnedRookPos)
	checkingRook := pieces.NewRook(models.Black, checkingRookPos)
	placePiece(t, gameBoard, whiteKing, whiteKingPos)
	placePiece(t, gameBoard, blackKing, blackKingPos)
	placePiece(t, gameBoard, pinnedRook, pinnedRookPos)
	placePiece(t, gameBoard, checkingRook, checkingRookPos)

	rules, err := NewRules(
		moves.NewMoveService(gameBoard),
		gameBoard,
		models.White,
		[]models.Piece{whiteKing, pinnedRook},
		[]models.Piece{blackKing, checkingRook},
		whiteKingPos,
		blackKingPos,
	)
	if err != nil {
		t.Fatalf("NewRules() error = %v", err)
	}

	legalMoves, err := rules.LegalMovesFor(pinnedRookPos)
	if err != nil {
		t.Fatalf("LegalMovesFor() error = %v", err)
	}

	illegalMove := models.NewPosition(models.Rank2, models.FileD)
	if hasMove(legalMoves, illegalMove) {
		t.Fatalf("LegalMovesFor() included pinned move %v: %v", illegalMove, legalMoves)
	}
}

func placePiece(t *testing.T, gameBoard *board.Board, piece models.Piece, pos models.Position) {
	t.Helper()

	gameBoard.SpotAt(pos).Piece = piece
}

func newEmptyBoard(t *testing.T) *board.Board {
	t.Helper()

	gameBoard := board.NewBoard()
	for rank := models.Rank1; rank <= models.Rank8; rank++ {
		for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
			gameBoard.Spots[rank.ToIndex()][file].Piece = nil
		}
	}
	return gameBoard
}

func hasMove(moves []models.Position, want models.Position) bool {
	for _, move := range moves {
		if move == want {
			return true
		}
	}
	return false
}
