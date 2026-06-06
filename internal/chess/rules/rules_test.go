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
		nil,
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

func TestLegalMovesForRejectsEnPassantThatExposesKing(t *testing.T) {
	gameBoard := newEmptyBoard(t)
	whiteKingPos := models.NewPosition(models.Rank5, models.FileH)
	blackKingPos := models.NewPosition(models.Rank8, models.FileH)
	whitePawnPos := models.NewPosition(models.Rank5, models.FileG)
	blackPawnPos := models.NewPosition(models.Rank5, models.FileF)
	blackRookPos := models.NewPosition(models.Rank5, models.FileA)
	enPassantTarget := models.NewPosition(models.Rank6, models.FileF)
	lastMove := models.NewMove(
		models.NewPosition(models.Rank7, models.FileF),
		blackPawnPos,
	)

	whiteKing := pieces.NewKing(models.White, whiteKingPos)
	blackKing := pieces.NewKing(models.Black, blackKingPos)
	whitePawn := pieces.NewPawn(models.White, whitePawnPos)
	blackPawn := pieces.NewPawn(models.Black, blackPawnPos)
	blackRook := pieces.NewRook(models.Black, blackRookPos)
	placePiece(t, gameBoard, whiteKing, whiteKingPos)
	placePiece(t, gameBoard, blackKing, blackKingPos)
	placePiece(t, gameBoard, whitePawn, whitePawnPos)
	placePiece(t, gameBoard, blackPawn, blackPawnPos)
	placePiece(t, gameBoard, blackRook, blackRookPos)

	rules, err := NewRules(
		moves.NewMoveService(gameBoard),
		gameBoard,
		models.White,
		[]models.Piece{whiteKing, whitePawn},
		[]models.Piece{blackKing, blackPawn, blackRook},
		whiteKingPos,
		blackKingPos,
		&lastMove,
	)
	if err != nil {
		t.Fatalf("NewRules() error = %v", err)
	}

	legalMoves, err := rules.LegalMovesFor(whitePawnPos)
	if err != nil {
		t.Fatalf("LegalMovesFor() error = %v", err)
	}
	if hasMove(legalMoves, enPassantTarget) {
		t.Fatalf("LegalMovesFor() included en passant move that exposes king: %v", legalMoves)
	}
}

func TestCurrentPlayerIsInCheck(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position)
		want  bool
	}{
		{
			name: "direct rook check",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				blackKingPos := models.NewPosition(models.Rank8, models.FileA)
				blackRookPos := models.NewPosition(models.Rank8, models.FileE)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				blackRook := pieces.NewRook(models.Black, blackRookPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)
				placePiece(t, gameBoard, blackRook, blackRookPos)

				return []models.Piece{whiteKing}, []models.Piece{blackKing, blackRook}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "blocked rook check",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				blackKingPos := models.NewPosition(models.Rank8, models.FileA)
				whiteBlockerPos := models.NewPosition(models.Rank2, models.FileE)
				blackRookPos := models.NewPosition(models.Rank8, models.FileE)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteBlocker := pieces.NewRook(models.White, whiteBlockerPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				blackRook := pieces.NewRook(models.Black, blackRookPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteBlocker, whiteBlockerPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)
				placePiece(t, gameBoard, blackRook, blackRookPos)

				return []models.Piece{whiteKing, whiteBlocker}, []models.Piece{blackKing, blackRook}, whiteKingPos, blackKingPos
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameBoard := newEmptyBoard(t)
			whitePieces, blackPieces, whiteKingPos, blackKingPos := tt.setup(gameBoard)
			rules := newRules(t, gameBoard, models.White, whitePieces, blackPieces, whiteKingPos, blackKingPos)

			if got := rules.CurrentPlayerIsInCheck(); got != tt.want {
				t.Fatalf("CurrentPlayerIsInCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCheckmate(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position)
		want  bool
	}{
		{
			name: "checked king has escape",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank6, models.FileE)
				whiteQueenPos := models.NewPosition(models.Rank7, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileH)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteQueen := pieces.NewQueen(models.White, whiteQueenPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteQueen, whiteQueenPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteQueen}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
		{
			name: "checked king has no legal moves",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank6, models.FileF)
				whiteQueenPos := models.NewPosition(models.Rank7, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileH)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteQueen := pieces.NewQueen(models.White, whiteQueenPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteQueen, whiteQueenPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteQueen}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "stalemate is not checkmate",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank7, models.FileF)
				whiteQueenPos := models.NewPosition(models.Rank6, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileH)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteQueen := pieces.NewQueen(models.White, whiteQueenPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteQueen, whiteQueenPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteQueen}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameBoard := newEmptyBoard(t)
			whitePieces, blackPieces, whiteKingPos, blackKingPos := tt.setup(gameBoard)
			rules := newRules(t, gameBoard, models.Black, whitePieces, blackPieces, whiteKingPos, blackKingPos)

			if got := rules.IsCheckmate(); got != tt.want {
				t.Fatalf("IsCheckmate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDraw(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position)
		want  bool
	}{
		{
			name: "insufficient material",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "stalemate",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank7, models.FileF)
				whiteQueenPos := models.NewPosition(models.Rank6, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileH)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteQueen := pieces.NewQueen(models.White, whiteQueenPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteQueen, whiteQueenPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteQueen}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "checkmate is not draw",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank6, models.FileF)
				whiteQueenPos := models.NewPosition(models.Rank7, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileH)

				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteQueen := pieces.NewQueen(models.White, whiteQueenPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteQueen, whiteQueenPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteQueen}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameBoard := newEmptyBoard(t)
			whitePieces, blackPieces, whiteKingPos, blackKingPos := tt.setup(gameBoard)
			rules := newRules(t, gameBoard, models.Black, whitePieces, blackPieces, whiteKingPos, blackKingPos)

			if got := rules.IsDraw(); got != tt.want {
				t.Fatalf("IsDraw() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInsufficientMaterial(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position)
		want  bool
	}{
		{
			name: "king against king",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "king and bishop against king",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteBishopPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteBishop := pieces.NewBishop(models.White, whiteBishopPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteBishop, whiteBishopPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteBishop}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "king and knight against king",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteKnightPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteKnight := pieces.NewKnight(models.White, whiteKnightPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteKnight, whiteKnightPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteKnight}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "bishops against bishops",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteBishopPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				blackBishopPos := models.NewPosition(models.Rank6, models.FileG)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteBishop := pieces.NewBishop(models.White, whiteBishopPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				blackBishop := pieces.NewBishop(models.Black, blackBishopPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteBishop, whiteBishopPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)
				placePiece(t, gameBoard, blackBishop, blackBishopPos)

				return []models.Piece{whiteKing, whiteBishop}, []models.Piece{blackKing, blackBishop}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "bishop against knight",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteBishopPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				blackKnightPos := models.NewPosition(models.Rank6, models.FileF)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteBishop := pieces.NewBishop(models.White, whiteBishopPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				blackKnight := pieces.NewKnight(models.Black, blackKnightPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteBishop, whiteBishopPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)
				placePiece(t, gameBoard, blackKnight, blackKnightPos)

				return []models.Piece{whiteKing, whiteBishop}, []models.Piece{blackKing, blackKnight}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "two knights against lone king",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteKnightOnePos := models.NewPosition(models.Rank2, models.FileC)
				whiteKnightTwoPos := models.NewPosition(models.Rank2, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteKnightOne := pieces.NewKnight(models.White, whiteKnightOnePos)
				whiteKnightTwo := pieces.NewKnight(models.White, whiteKnightTwoPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteKnightOne, whiteKnightOnePos)
				placePiece(t, gameBoard, whiteKnightTwo, whiteKnightTwoPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteKnightOne, whiteKnightTwo}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
			want: true,
		},
		{
			name: "two knights against king and bishop can continue",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteKnightOnePos := models.NewPosition(models.Rank2, models.FileC)
				whiteKnightTwoPos := models.NewPosition(models.Rank2, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				blackBishopPos := models.NewPosition(models.Rank6, models.FileF)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteKnightOne := pieces.NewKnight(models.White, whiteKnightOnePos)
				whiteKnightTwo := pieces.NewKnight(models.White, whiteKnightTwoPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				blackBishop := pieces.NewBishop(models.Black, blackBishopPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteKnightOne, whiteKnightOnePos)
				placePiece(t, gameBoard, whiteKnightTwo, whiteKnightTwoPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)
				placePiece(t, gameBoard, blackBishop, blackBishopPos)

				return []models.Piece{whiteKing, whiteKnightOne, whiteKnightTwo}, []models.Piece{blackKing, blackBishop}, whiteKingPos, blackKingPos
			},
		},
		{
			name: "rook can mate",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteRookPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteRook := pieces.NewRook(models.White, whiteRookPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteRook, whiteRookPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteRook}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
		{
			name: "pawn can promote",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whitePawnPos := models.NewPosition(models.Rank2, models.FileC)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whitePawn := pieces.NewPawn(models.White, whitePawnPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whitePawn, whitePawnPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whitePawn}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
		{
			name: "two minor pieces can mate",
			setup: func(gameBoard *board.Board) ([]models.Piece, []models.Piece, models.Position, models.Position) {
				whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
				whiteBishopPos := models.NewPosition(models.Rank2, models.FileC)
				whiteKnightPos := models.NewPosition(models.Rank2, models.FileG)
				blackKingPos := models.NewPosition(models.Rank8, models.FileE)
				whiteKing := pieces.NewKing(models.White, whiteKingPos)
				whiteBishop := pieces.NewBishop(models.White, whiteBishopPos)
				whiteKnight := pieces.NewKnight(models.White, whiteKnightPos)
				blackKing := pieces.NewKing(models.Black, blackKingPos)
				placePiece(t, gameBoard, whiteKing, whiteKingPos)
				placePiece(t, gameBoard, whiteBishop, whiteBishopPos)
				placePiece(t, gameBoard, whiteKnight, whiteKnightPos)
				placePiece(t, gameBoard, blackKing, blackKingPos)

				return []models.Piece{whiteKing, whiteBishop, whiteKnight}, []models.Piece{blackKing}, whiteKingPos, blackKingPos
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameBoard := newEmptyBoard(t)
			whitePieces, blackPieces, whiteKingPos, blackKingPos := tt.setup(gameBoard)
			rules := newRules(t, gameBoard, models.White, whitePieces, blackPieces, whiteKingPos, blackKingPos)

			if got := rules.InsufficientMaterial(); got != tt.want {
				t.Fatalf("InsufficientMaterial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newRules(
	t *testing.T,
	gameBoard *board.Board,
	turn models.Color,
	whitePieces []models.Piece,
	blackPieces []models.Piece,
	whiteKingPos models.Position,
	blackKingPos models.Position,
) Rules {
	t.Helper()

	rules, err := NewRules(
		moves.NewMoveService(gameBoard),
		gameBoard,
		turn,
		whitePieces,
		blackPieces,
		whiteKingPos,
		blackKingPos,
		nil,
	)
	if err != nil {
		t.Fatalf("NewRules() error = %v", err)
	}
	return rules
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
