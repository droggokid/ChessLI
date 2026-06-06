package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestPawnPossibleMoves(t *testing.T) {
	tests := []struct {
		name  string
		color models.Color
		from  models.Position
		place func(*testBoard)
		want  []models.Position
	}{
		{
			name:  "white can move one or two squares from starting rank",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank3, models.FileE),
				models.NewPosition(models.Rank4, models.FileE),
			},
		},
		{
			name:  "black can move one or two squares from starting rank",
			color: models.Black,
			from:  models.NewPosition(models.Rank7, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank6, models.FileE),
				models.NewPosition(models.Rank5, models.FileE),
			},
		},
		{
			name:  "cannot move forward when blocked",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank3, models.FileE)), models.NewPosition(models.Rank3, models.FileE))
			},
		},
		{
			name:  "cannot move two squares when second square is blocked",
			color: models.White,
			from:  models.NewPosition(models.Rank2, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank4, models.FileE)), models.NewPosition(models.Rank4, models.FileE))
			},
			want: []models.Position{
				models.NewPosition(models.Rank3, models.FileE),
			},
		},
		{
			name:  "can capture diagonally",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileD)), models.NewPosition(models.Rank5, models.FileD))
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileF)), models.NewPosition(models.Rank5, models.FileF))
			},
			want: []models.Position{
				models.NewPosition(models.Rank5, models.FileE),
				models.NewPosition(models.Rank5, models.FileD),
				models.NewPosition(models.Rank5, models.FileF),
			},
		},
		{
			name:  "cannot capture forward or move diagonally to empty square",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			place: func(board *testBoard) {
				board.place(NewPawn(models.Black, models.NewPosition(models.Rank5, models.FileE)), models.NewPosition(models.Rank5, models.FileE))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			pawn := NewPawn(tt.color, tt.from)
			board.place(pawn, tt.from)

			if tt.place != nil {
				tt.place(board)
			}

			assertMoves(t, pawn.PossibleMoves(board, nil), tt.want...)
		})
	}
}

func TestPawnEnPassantMove(t *testing.T) {
	tests := []struct {
		name      string
		color     models.Color
		from      models.Position
		lastMove  *models.Move
		placeLast func(*testBoard)
		occupy    *models.Position
		want      models.Position
		wantOK    bool
	}{
		{
			name:  "white captures black pawn en passant",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank7, models.FileD),
				To:   pos(models.Rank5, models.FileD),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.Black, models.Rank5, models.FileD)
			},
			want:   pos(models.Rank6, models.FileD),
			wantOK: true,
		},
		{
			name:  "black captures white pawn en passant",
			color: models.Black,
			from:  pos(models.Rank4, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank2, models.FileD),
				To:   pos(models.Rank4, models.FileD),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.White, models.Rank4, models.FileD)
			},
			want:   pos(models.Rank3, models.FileD),
			wantOK: true,
		},
		{
			name:   "no previous move",
			color:  models.White,
			from:   pos(models.Rank5, models.FileE),
			wantOK: false,
		},
		{
			name:  "pawn moved only one rank",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank6, models.FileD),
				To:   pos(models.Rank5, models.FileD),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.Black, models.Rank5, models.FileD)
			},
			wantOK: false,
		},
		{
			name:  "last moved pawn has same color",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank3, models.FileD),
				To:   pos(models.Rank5, models.FileD),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.White, models.Rank5, models.FileD)
			},
			wantOK: false,
		},
		{
			name:  "last moved pawn is not beside capturing pawn",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank7, models.FileB),
				To:   pos(models.Rank5, models.FileB),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.Black, models.Rank5, models.FileB)
			},
			wantOK: false,
		},
		{
			name:  "en passant expires after another piece moves",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank8, models.FileA),
				To:   pos(models.Rank7, models.FileA),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.Black, models.Rank5, models.FileD)
				rookPosition := pos(models.Rank7, models.FileA)
				board.place(NewRook(models.Black, rookPosition), rookPosition)
			},
			wantOK: false,
		},
		{
			name:  "passed square is occupied",
			color: models.White,
			from:  pos(models.Rank5, models.FileE),
			lastMove: &models.Move{
				From: pos(models.Rank7, models.FileD),
				To:   pos(models.Rank5, models.FileD),
			},
			placeLast: func(board *testBoard) {
				board.placePawn(models.Black, models.Rank5, models.FileD)
			},
			occupy: positionPointer(pos(models.Rank6, models.FileD)),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			pawn := NewPawn(tt.color, tt.from).(*Pawn)
			board.place(pawn, tt.from)

			if tt.placeLast != nil {
				tt.placeLast(board)
			}
			if tt.occupy != nil {
				board.placePawn(models.Black, tt.occupy.Rank, tt.occupy.File)
			}

			got, ok := pawn.enPassantMove(board, tt.lastMove)
			if ok != tt.wantOK {
				t.Fatalf("enPassantMove() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("enPassantMove() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPawnPossibleMovesIncludesEnPassant(t *testing.T) {
	board := newTestBoard()
	from := pos(models.Rank5, models.FileE)
	pawn := NewPawn(models.White, from)
	board.place(pawn, from)
	board.placePawn(models.Black, models.Rank5, models.FileD)

	lastMove := &models.Move{
		From: pos(models.Rank7, models.FileD),
		To:   pos(models.Rank5, models.FileD),
	}

	assertMoves(t, pawn.PossibleMoves(board, lastMove),
		pos(models.Rank6, models.FileE),
		pos(models.Rank6, models.FileD),
	)
}

func TestPawnAttackedSquares(t *testing.T) {
	tests := []struct {
		name  string
		color models.Color
		from  models.Position
		want  []models.Position
	}{
		{
			name:  "white attacks diagonally north",
			color: models.White,
			from:  models.NewPosition(models.Rank4, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank5, models.FileD),
				models.NewPosition(models.Rank5, models.FileF),
			},
		},
		{
			name:  "black attacks diagonally south",
			color: models.Black,
			from:  models.NewPosition(models.Rank5, models.FileE),
			want: []models.Position{
				models.NewPosition(models.Rank4, models.FileD),
				models.NewPosition(models.Rank4, models.FileF),
			},
		},
		{
			name:  "edge pawn only attacks in-bounds squares",
			color: models.White,
			from:  models.NewPosition(models.Rank7, models.FileA),
			want: []models.Position{
				models.NewPosition(models.Rank8, models.FileB),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := newTestBoard()
			pawn := NewPawn(tt.color, tt.from)
			board.place(pawn, tt.from)

			assertMoves(t, pawn.AttackedSquares(board), tt.want...)
		})
	}
}
