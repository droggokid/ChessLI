package moves

import (
	"testing"

	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
)

func TestResolveMoveReturnsBoardSpotsAndPieces(t *testing.T) {
	gameBoard := newEmptyBoard(t)
	from := models.NewPosition(models.Rank1, models.FileA)
	to := models.NewPosition(models.Rank8, models.FileA)
	rook := pieces.NewRook(models.White, from)
	captured := pieces.NewKnight(models.Black, to)
	placePiece(t, gameBoard, rook, from)
	placePiece(t, gameBoard, captured, to)

	service := NewMoveService(gameBoard)
	resolved, err := service.ResolveMove(models.NewMove(from, to))
	if err != nil {
		t.Fatalf("ResolveMove() error = %v", err)
	}

	if resolved.FromSpot != gameBoard.SpotAt(from) {
		t.Fatal("ResolveMove() did not keep the source board spot")
	}
	if resolved.ToSpot != gameBoard.SpotAt(to) {
		t.Fatal("ResolveMove() did not keep the target board spot")
	}
	if resolved.MovingPiece != rook {
		t.Fatalf("MovingPiece = %v, want %v", resolved.MovingPiece, rook)
	}
	if resolved.CapturedPiece != captured {
		t.Fatalf("CapturedPiece = %v, want %v", resolved.CapturedPiece, captured)
	}
}

func TestResolveMoveReturnsError(t *testing.T) {
	tests := []struct {
		name string
		from models.Position
		to   models.Position
		want string
	}{
		{
			name: "empty source",
			from: models.NewPosition(models.Rank1, models.FileA),
			to:   models.NewPosition(models.Rank2, models.FileA),
			want: "no piece at source position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gameBoard := newEmptyBoard(t)
			service := NewMoveService(gameBoard)

			_, err := service.ResolveMove(models.NewMove(tt.from, tt.to))
			if err == nil {
				t.Fatal("ResolveMove() error = nil, want error")
			}
			if err.Error() != tt.want {
				t.Fatalf("ResolveMove() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestApplyAndRevertMoveRestoresBoard(t *testing.T) {
	gameBoard := newEmptyBoard(t)
	from := models.NewPosition(models.Rank1, models.FileA)
	to := models.NewPosition(models.Rank8, models.FileA)
	rook := pieces.NewRook(models.White, from)
	captured := pieces.NewKnight(models.Black, to)
	placePiece(t, gameBoard, rook, from)
	placePiece(t, gameBoard, captured, to)

	service := NewMoveService(gameBoard)
	resolved, err := service.ResolveMove(models.NewMove(from, to))
	if err != nil {
		t.Fatalf("ResolveMove() error = %v", err)
	}

	service.ApplyMove(resolved)
	if gameBoard.SpotAt(from).Piece != nil {
		t.Fatal("ApplyMove() left the source spot occupied")
	}
	if gameBoard.SpotAt(to).Piece != rook {
		t.Fatal("ApplyMove() did not move the piece to the target spot")
	}
	if rook.Position() != to {
		t.Fatalf("rook.Position() = %v, want %v", rook.Position(), to)
	}

	service.RevertMove(resolved)
	if gameBoard.SpotAt(from).Piece != rook {
		t.Fatal("RevertMove() did not restore the source spot")
	}
	if gameBoard.SpotAt(to).Piece != captured {
		t.Fatal("RevertMove() did not restore the captured piece")
	}
	if rook.Position() != from {
		t.Fatalf("rook.Position() = %v, want %v", rook.Position(), from)
	}
}

func TestApplyAndRevertEnPassantMoveRestoresBoard(t *testing.T) {
	gameBoard := newEmptyBoard(t)
	from := models.NewPosition(models.Rank5, models.FileE)
	to := models.NewPosition(models.Rank6, models.FileD)
	capturedPosition := models.NewPosition(models.Rank5, models.FileD)
	whitePawn := pieces.NewPawn(models.White, from)
	blackPawn := pieces.NewPawn(models.Black, capturedPosition)
	placePiece(t, gameBoard, whitePawn, from)
	placePiece(t, gameBoard, blackPawn, capturedPosition)

	service := NewMoveService(gameBoard)
	resolved, err := service.ResolveMove(models.NewMove(from, to))
	if err != nil {
		t.Fatalf("ResolveMove() error = %v", err)
	}

	if !resolved.WasEnPassant {
		t.Fatal("WasEnPassant = false, want true")
	}
	if resolved.CapturedPiece != blackPawn {
		t.Fatalf("CapturedPiece = %v, want %v", resolved.CapturedPiece, blackPawn)
	}
	if resolved.CapturedSpot != gameBoard.SpotAt(capturedPosition) {
		t.Fatal("CapturedSpot does not reference the adjacent pawn spot")
	}

	service.ApplyMove(resolved)
	if gameBoard.SpotAt(from).Piece != nil {
		t.Fatal("ApplyMove() left the source spot occupied")
	}
	if gameBoard.SpotAt(to).Piece != whitePawn {
		t.Fatal("ApplyMove() did not move the pawn to the target spot")
	}
	if gameBoard.SpotAt(capturedPosition).Piece != nil {
		t.Fatal("ApplyMove() did not remove the en passant pawn")
	}

	service.RevertMove(resolved)
	if gameBoard.SpotAt(from).Piece != whitePawn {
		t.Fatal("RevertMove() did not restore the moving pawn")
	}
	if gameBoard.SpotAt(to).Piece != nil {
		t.Fatal("RevertMove() left the en passant target occupied")
	}
	if gameBoard.SpotAt(capturedPosition).Piece != blackPawn {
		t.Fatal("RevertMove() did not restore the captured pawn")
	}
	if whitePawn.Position() != from {
		t.Fatalf("whitePawn.Position() = %v, want %v", whitePawn.Position(), from)
	}
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

func placePiece(t *testing.T, gameBoard *board.Board, piece models.Piece, pos models.Position) {
	t.Helper()

	gameBoard.SpotAt(pos).Piece = piece
}
