package game

import (
	"testing"

	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"chessli/internal/chess/moves"
)

func TestLegalMovesForKingSafety(t *testing.T) {
	tests := []struct {
		name      string
		from      models.Position
		candidate models.Position
		setup     func(*Game)
		wantLegal bool
	}{
		{
			name:      "king cannot move into attacked square",
			from:      models.NewPosition(models.Rank1, models.FileE),
			candidate: models.NewPosition(models.Rank2, models.FileE),
			setup: func(game *Game) {
				placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileE)), models.NewPosition(models.Rank1, models.FileE))
				placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
				placePiece(game, pieces.NewRook(models.Black, models.NewPosition(models.Rank8, models.FileE)), models.NewPosition(models.Rank8, models.FileE))
			},
		},
		{
			name:      "pinned piece cannot expose king",
			from:      models.NewPosition(models.Rank2, models.FileE),
			candidate: models.NewPosition(models.Rank2, models.FileD),
			setup: func(game *Game) {
				placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileE)), models.NewPosition(models.Rank1, models.FileE))
				placePiece(game, pieces.NewRook(models.White, models.NewPosition(models.Rank2, models.FileE)), models.NewPosition(models.Rank2, models.FileE))
				placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
				placePiece(game, pieces.NewRook(models.Black, models.NewPosition(models.Rank8, models.FileE)), models.NewPosition(models.Rank8, models.FileE))
			},
		},
		{
			name:      "move that blocks check is legal",
			from:      models.NewPosition(models.Rank2, models.FileA),
			candidate: models.NewPosition(models.Rank2, models.FileE),
			setup: func(game *Game) {
				placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileE)), models.NewPosition(models.Rank1, models.FileE))
				placePiece(game, pieces.NewRook(models.White, models.NewPosition(models.Rank2, models.FileA)), models.NewPosition(models.Rank2, models.FileA))
				placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
				placePiece(game, pieces.NewRook(models.Black, models.NewPosition(models.Rank8, models.FileE)), models.NewPosition(models.Rank8, models.FileE))
			},
			wantLegal: true,
		},
		{
			name:      "move that captures checking piece is legal",
			from:      models.NewPosition(models.Rank8, models.FileA),
			candidate: models.NewPosition(models.Rank8, models.FileE),
			setup: func(game *Game) {
				placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileE)), models.NewPosition(models.Rank1, models.FileE))
				placePiece(game, pieces.NewRook(models.White, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
				placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileH)), models.NewPosition(models.Rank8, models.FileH))
				placePiece(game, pieces.NewRook(models.Black, models.NewPosition(models.Rank8, models.FileE)), models.NewPosition(models.Rank8, models.FileE))
			},
			wantLegal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := newEmptyGame(models.White)
			tt.setup(game)
			refreshGameState(t, game)

			moves, err := game.LegalMovesFor(tt.from)
			if err != nil {
				t.Fatalf("LegalMovesFor() error = %v", err)
			}

			gotLegal := hasMove(moves, tt.candidate)
			if gotLegal != tt.wantLegal {
				t.Fatalf("hasMove(%v) = %v, want %v; moves = %v", tt.candidate, gotLegal, tt.wantLegal, moves)
			}
		})
	}
}

func TestCalculateAttackedSquaresSkipsCapturedPieceLeftInSlice(t *testing.T) {
	game := newEmptyGame(models.White)
	blackRookPos := models.NewPosition(models.Rank8, models.FileE)

	blackRook := pieces.NewRook(models.Black, blackRookPos)
	whiteRook := pieces.NewRook(models.White, blackRookPos)
	placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileA)), models.NewPosition(models.Rank1, models.FileA))
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileH)), models.NewPosition(models.Rank8, models.FileH))
	placePiece(game, blackRook, blackRookPos)

	game.Board.SpotAt(blackRookPos).Piece = whiteRook

	attacked := game.rules.CalculateAttackedSquares(models.Black)
	if attacked[models.NewPosition(models.Rank1, models.FileE)] {
		t.Fatalf("captured rook still contributed attacks: %v", attacked)
	}
}

func TestMoveUpdatesKingPosition(t *testing.T) {
	game := newEmptyGame(models.White)
	from := models.NewPosition(models.Rank1, models.FileE)
	to := models.NewPosition(models.Rank2, models.FileE)
	placePiece(game, pieces.NewKing(models.White, from), from)
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
	refreshGameState(t, game)

	if err := game.Move(models.NewMove(from, to)); err != nil {
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

func TestMoveCapturesPieceAndRecordsHistory(t *testing.T) {
	game := newEmptyGame(models.White)
	from := models.NewPosition(models.Rank1, models.FileA)
	to := models.NewPosition(models.Rank8, models.FileA)
	whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
	blackKingPos := models.NewPosition(models.Rank8, models.FileH)
	rook := pieces.NewRook(models.White, from)
	capturedKnight := pieces.NewKnight(models.Black, to)
	placePiece(game, pieces.NewKing(models.White, whiteKingPos), whiteKingPos)
	placePiece(game, rook, from)
	placePiece(game, pieces.NewKing(models.Black, blackKingPos), blackKingPos)
	placePiece(game, capturedKnight, to)
	refreshGameState(t, game)

	move := models.NewMove(from, to)
	if err := game.Move(move); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if len(game.CapturedByWhite) != 1 {
		t.Fatalf("len(CapturedByWhite) = %d, want 1", len(game.CapturedByWhite))
	}
	if game.CapturedByWhite[0] != capturedKnight {
		t.Fatalf("CapturedByWhite[0] = %v, want %v", game.CapturedByWhite[0], capturedKnight)
	}
	if containsPiece(game.BlackPieces, capturedKnight) {
		t.Fatal("captured piece still exists in BlackPieces")
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

func TestMoveCanCreateDiscoveredCheck(t *testing.T) {
	game := newEmptyGame(models.White)
	whiteKingPos := models.NewPosition(models.Rank1, models.FileA)
	blackKingPos := models.NewPosition(models.Rank8, models.FileE)
	whiteRookPos := models.NewPosition(models.Rank1, models.FileE)
	whiteBishopFrom := models.NewPosition(models.Rank2, models.FileE)
	whiteBishopTo := models.NewPosition(models.Rank3, models.FileD)

	placePiece(game, pieces.NewKing(models.White, whiteKingPos), whiteKingPos)
	placePiece(game, pieces.NewRook(models.White, whiteRookPos), whiteRookPos)
	placePiece(game, pieces.NewBishop(models.White, whiteBishopFrom), whiteBishopFrom)
	placePiece(game, pieces.NewKing(models.Black, blackKingPos), blackKingPos)
	refreshGameState(t, game)

	if game.State != GameStateActive {
		t.Fatalf("State = %v, want %v before discovered check move", game.State, GameStateActive)
	}

	if err := game.Move(models.NewMove(whiteBishopFrom, whiteBishopTo)); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if !game.CurrentPlayerIsInCheck() {
		t.Fatal("current player is not in check after discovered check move")
	}
	if game.State != GameStateCheck {
		t.Fatalf("State = %v, want %v after discovered check move", game.State, GameStateCheck)
	}
}

func TestMoveCanEscapeCheckAndGiveCheck(t *testing.T) {
	game := newEmptyGame(models.White)
	whiteKingPos := models.NewPosition(models.Rank1, models.FileE)
	whiteRookFrom := models.NewPosition(models.Rank2, models.FileA)
	whiteRookTo := models.NewPosition(models.Rank2, models.FileE)
	blackKingPos := models.NewPosition(models.Rank2, models.FileH)
	blackRookPos := models.NewPosition(models.Rank8, models.FileE)

	placePiece(game, pieces.NewKing(models.White, whiteKingPos), whiteKingPos)
	placePiece(game, pieces.NewRook(models.White, whiteRookFrom), whiteRookFrom)
	placePiece(game, pieces.NewKing(models.Black, blackKingPos), blackKingPos)
	placePiece(game, pieces.NewRook(models.Black, blackRookPos), blackRookPos)
	refreshGameState(t, game)

	if !game.CurrentPlayerIsInCheck() {
		t.Fatal("current player is not in check before move")
	}
	if game.State != GameStateCheck {
		t.Fatalf("State = %v, want %v before move", game.State, GameStateCheck)
	}

	if err := game.Move(models.NewMove(whiteRookFrom, whiteRookTo)); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if !game.CurrentPlayerIsInCheck() {
		t.Fatal("current player is not in check after white escapes check")
	}
	if game.State != GameStateCheck {
		t.Fatalf("State = %v, want %v after white escapes check", game.State, GameStateCheck)
	}
}

func newEmptyGame(turn models.Color) *Game {
	gameBoard := board.NewBoard()
	for rank := models.Rank1; rank <= models.Rank8; rank++ {
		for file := models.FileA.ToIndex(); file <= models.FileH.ToIndex(); file++ {
			gameBoard.Spots[rank.ToIndex()][file].Piece = nil
		}
	}

	return &Game{
		moveService:       moves.NewMoveService(gameBoard),
		Board:             gameBoard,
		Turn:              turn,
		WhitePlayer:       NewPlayer("white", models.White),
		BlackPlayer:       NewPlayer("black", models.Black),
		WhitePieces:       make([]models.Piece, 0, 16),
		BlackPieces:       make([]models.Piece, 0, 16),
		CapturedByWhite:   make([]models.Piece, 0, 16),
		CapturedByBlack:   make([]models.Piece, 0, 16),
		WhiteKingPosition: models.NewPosition(models.Rank1, models.FileE),
		BlackKingPosition: models.NewPosition(models.Rank8, models.FileE),
		MoveHistory:       make([]MoveRecord, 0),
	}
}

func placePiece(game *Game, piece models.Piece, pos models.Position) {
	game.Board.SpotAt(pos).Piece = piece
	if piece.Color() == models.White {
		game.WhitePieces = append(game.WhitePieces, piece)
	} else {
		game.BlackPieces = append(game.BlackPieces, piece)
	}

	if _, ok := piece.(*pieces.King); ok {
		if piece.Color() == models.White {
			game.WhiteKingPosition = pos
		} else {
			game.BlackKingPosition = pos
		}
	}
}

func refreshGameState(t *testing.T, game *Game) {
	t.Helper()

	var err error
	game.rules, err = game.prepareRules()
	if err != nil {
		t.Fatalf("prepareRules() error = %v", err)
	}
	game.updateState()
}

func hasMove(moves []models.Position, want models.Position) bool {
	for _, move := range moves {
		if move == want {
			return true
		}
	}
	return false
}

func containsPiece(pieces []models.Piece, want models.Piece) bool {
	for _, piece := range pieces {
		if piece == want {
			return true
		}
	}
	return false
}
