package board

import (
	"strings"
	"testing"

	"chessli/internal/chess/board/models"
)

func TestNewBoardCreatesSpotsWithAlternatingColors(t *testing.T) {
	gameBoard := NewBoard()

	tests := []struct {
		name string
		pos  models.Position
		want models.Color
	}{
		{name: "a1 is black", pos: models.NewPosition(models.Rank1, models.FileA), want: models.Black},
		{name: "b1 is white", pos: models.NewPosition(models.Rank1, models.FileB), want: models.White},
		{name: "a2 is white", pos: models.NewPosition(models.Rank2, models.FileA), want: models.White},
		{name: "h8 is black", pos: models.NewPosition(models.Rank8, models.FileH), want: models.Black},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spot := gameBoard.SpotAt(tt.pos)
			if spot == nil {
				t.Fatalf("SpotAt(%v) = nil", tt.pos)
			}
			if spot.Position != tt.pos {
				t.Fatalf("spot.Position = %v, want %v", spot.Position, tt.pos)
			}
			if spot.Color != tt.want {
				t.Fatalf("spot.Color = %v, want %v", spot.Color, tt.want)
			}
		})
	}
}

func TestSpotAtReturnsNilForInvalidPosition(t *testing.T) {
	gameBoard := NewBoard()

	tests := []models.Position{
		models.NewPosition(models.Rank(-1), models.FileA),
		models.NewPosition(models.Rank8, models.File('i')),
	}

	for _, pos := range tests {
		t.Run(pos.String(), func(t *testing.T) {
			if spot := gameBoard.SpotAt(pos); spot != nil {
				t.Fatalf("SpotAt(%v) = %v, want nil", pos, spot)
			}
		})
	}
}

func TestNewBoardPlacesStartingPieces(t *testing.T) {
	gameBoard := NewBoard()

	tests := []struct {
		name      string
		pos       models.Position
		wantType  models.PieceType
		wantColor models.Color
	}{
		{name: "white rook on a1", pos: models.NewPosition(models.Rank1, models.FileA), wantType: models.Rook, wantColor: models.White},
		{name: "white king on e1", pos: models.NewPosition(models.Rank1, models.FileE), wantType: models.King, wantColor: models.White},
		{name: "white pawn on h2", pos: models.NewPosition(models.Rank2, models.FileH), wantType: models.Pawn, wantColor: models.White},
		{name: "black queen on d8", pos: models.NewPosition(models.Rank8, models.FileD), wantType: models.Queen, wantColor: models.Black},
		{name: "black pawn on a7", pos: models.NewPosition(models.Rank7, models.FileA), wantType: models.Pawn, wantColor: models.Black},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			piece := gameBoard.SpotAt(tt.pos).Piece
			if piece == nil {
				t.Fatalf("piece at %v = nil", tt.pos)
			}
			if piece.Type() != tt.wantType {
				t.Fatalf("piece.Type() = %v, want %v", piece.Type(), tt.wantType)
			}
			if piece.Color() != tt.wantColor {
				t.Fatalf("piece.Color() = %v, want %v", piece.Color(), tt.wantColor)
			}
			if piece.Position() != tt.pos {
				t.Fatalf("piece.Position() = %v, want %v", piece.Position(), tt.pos)
			}
		})
	}
}

func TestStarterPieces(t *testing.T) {
	gameBoard := NewBoard()

	assertStarterPieces(t, gameBoard.WhiteStarterPieces(), models.White)
	assertStarterPieces(t, gameBoard.BlackStarterPieces(), models.Black)
}

func TestBoardStringViews(t *testing.T) {
	gameBoard := NewBoard()

	whiteView := strings.Split(gameBoard.String(), "\n")
	if len(whiteView) != 64 {
		t.Fatalf("len(whiteView) = %d, want 64", len(whiteView))
	}
	if !strings.Contains(whiteView[0], "a8 black rook") {
		t.Fatalf("whiteView[0] = %q, want a8 black rook", whiteView[0])
	}
	if !strings.Contains(whiteView[63], "h1 white rook") {
		t.Fatalf("whiteView[63] = %q, want h1 white rook", whiteView[63])
	}

	blackView := strings.Split(gameBoard.StringBlackView(), "\n")
	if len(blackView) != 64 {
		t.Fatalf("len(blackView) = %d, want 64", len(blackView))
	}
	if !strings.Contains(blackView[0], "a1 white rook") {
		t.Fatalf("blackView[0] = %q, want a1 white rook", blackView[0])
	}
	if !strings.Contains(blackView[63], "h8 black rook") {
		t.Fatalf("blackView[63] = %q, want h8 black rook", blackView[63])
	}
}

func assertStarterPieces(t *testing.T, pieces []models.Piece, color models.Color) {
	t.Helper()

	if len(pieces) != 16 {
		t.Fatalf("len(pieces) = %d, want 16", len(pieces))
	}

	for _, piece := range pieces {
		if piece == nil {
			t.Fatal("starter pieces contained nil")
		}
		if piece.Color() != color {
			t.Fatalf("piece.Color() = %v, want %v", piece.Color(), color)
		}
	}
}
