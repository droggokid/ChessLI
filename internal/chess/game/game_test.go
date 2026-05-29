package game

import (
	"testing"

	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
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
			refreshGameState(game)

			gotLegal := hasMove(game.LegalMovesFor(tt.from), tt.candidate)
			if gotLegal != tt.wantLegal {
				t.Fatalf("hasMove(%v) = %v, want %v; moves = %v", tt.candidate, gotLegal, tt.wantLegal, game.LegalMovesFor(tt.from))
			}
		})
	}
}

func TestMoveUpdatesKingPosition(t *testing.T) {
	game := newEmptyGame(models.White)
	from := models.NewPosition(models.Rank1, models.FileE)
	to := models.NewPosition(models.Rank2, models.FileE)

	placePiece(game, pieces.NewKing(models.White, from), from)
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileA)), models.NewPosition(models.Rank8, models.FileA))
	refreshGameState(game)

	if err := game.Move(from, to); err != nil {
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

func TestCalculateAttackedSquaresSkipsCapturedPieceLeftInSlice(t *testing.T) {
	game := newEmptyGame(models.White)
	blackRookPos := models.NewPosition(models.Rank8, models.FileE)

	blackRook := pieces.NewRook(models.Black, blackRookPos)
	whiteRook := pieces.NewRook(models.White, blackRookPos)
	placePiece(game, pieces.NewKing(models.White, models.NewPosition(models.Rank1, models.FileA)), models.NewPosition(models.Rank1, models.FileA))
	placePiece(game, pieces.NewKing(models.Black, models.NewPosition(models.Rank8, models.FileH)), models.NewPosition(models.Rank8, models.FileH))
	placePiece(game, blackRook, blackRookPos)

	game.Board.SpotAt(blackRookPos).Piece = whiteRook

	attacked := game.CalculateAttackedSquares(models.Black)
	if attacked[models.NewPosition(models.Rank1, models.FileE)] {
		t.Fatalf("captured rook still contributed attacks: %v", attacked)
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
		legalMoves:        make(map[models.Position][]models.Position),
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

func refreshGameState(game *Game) {
	game.legalMoves = make(map[models.Position][]models.Position)
	game.SquaresAttackedByWhite = game.CalculateAttackedSquares(models.White)
	game.SquaresAttackedByBlack = game.CalculateAttackedSquares(models.Black)
	game.legalMoves = game.CalculateAllLegalMoves()
}

func hasMove(moves []models.Position, want models.Position) bool {
	for _, move := range moves {
		if move == want {
			return true
		}
	}
	return false
}
