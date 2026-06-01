// Package game owns players, turns, captures, move orchestration, and check safety.
package game

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"errors"
)

// Game contains the mutable state for one chess game.
type Game struct {
	Board *board.Board
	Turn  models.Color

	WhitePlayer Player
	BlackPlayer Player

	WhitePieces []models.Piece
	BlackPieces []models.Piece

	CapturedByWhite []models.Piece
	CapturedByBlack []models.Piece

	SquaresAttackedByWhite map[models.Position]bool
	SquaresAttackedByBlack map[models.Position]bool

	WhiteKingPosition models.Position
	BlackKingPosition models.Position

	MoveHistory []MoveRecord

	legalMoves map[models.Position][]models.Position
}

// NewGame creates a standard chess game with two named players.
func NewGame(player1 string, player2 string) Game {
	gameBoard := board.NewBoard()
	game := Game{
		Board:             gameBoard,
		Turn:              models.White,
		WhitePlayer:       NewPlayer(player1, models.White),
		BlackPlayer:       NewPlayer(player2, models.Black),
		WhitePieces:       gameBoard.WhiteStarterPieces(),
		BlackPieces:       gameBoard.BlackStarterPieces(),
		CapturedByWhite:   make([]models.Piece, 0, 16),
		CapturedByBlack:   make([]models.Piece, 0, 16),
		WhiteKingPosition: models.NewPosition(models.Rank1, models.FileE),
		BlackKingPosition: models.NewPosition(models.Rank8, models.FileE),
		MoveHistory:       make([]MoveRecord, 0),
		legalMoves:        make(map[models.Position][]models.Position),
	}
	game.SquaresAttackedByWhite = game.CalculateAttackedSquares(models.White)
	game.SquaresAttackedByBlack = game.CalculateAttackedSquares(models.Black)

	game.legalMoves, _ = game.CalculateAllLegalMoves()

	return game
}

// CalculateAttackedSquares returns the set of squares controlled by the given color.
func (g *Game) CalculateAttackedSquares(by models.Color) map[models.Position]bool {
	attackedSquares := make(map[models.Position]bool)

	for _, piece := range g.piecesFor(by) {
		// Simulated captures mutate the board without removing pieces from the
		// piece slices. Trust the board as the source of truth before asking a
		// piece for attacks.
		spot := g.Board.SpotAt(piece.Position())
		if spot == nil || spot.Piece != piece {
			continue
		}

		for _, pos := range piece.AttackedSquares(g.Board) {
			attackedSquares[pos] = true
		}
	}

	return attackedSquares
}

func (g *Game) piecesFor(color models.Color) []models.Piece {
	if color == models.White {
		return g.WhitePieces
	}
	return g.BlackPieces
}

// CalculateAllLegalMoves calculates legal moves for the current player.
func (g *Game) CalculateAllLegalMoves() (map[models.Position][]models.Position, error) {
	legalMoves := make(map[models.Position][]models.Position)

	var err error
	if g.Turn == models.White {
		for _, piece := range g.WhitePieces {
			legalMoves[piece.Position()], err = g.LegalMovesFor(piece.Position())
			if err != nil {
				return nil, err
			}
		}
	} else {
		for _, piece := range g.BlackPieces {
			legalMoves[piece.Position()], err = g.LegalMovesFor(piece.Position())
			if err != nil {
				return nil, err
			}
		}
	}

	return legalMoves, nil
}

// LegalMovesFor returns legal moves for the current player's piece at position.
func (g *Game) LegalMovesFor(position models.Position) ([]models.Position, error) {
	if moves, ok := g.legalMoves[position]; ok {
		return moves, nil
	}

	fromSpot := g.Board.SpotAt(position)
	if fromSpot == nil {
		return nil, errors.New("no legal moves found")
	}

	movingPiece := fromSpot.Piece
	if movingPiece == nil || movingPiece.Color() != g.Turn {
		return nil, errors.New("no legal moves found")
	}

	possibleMoves := movingPiece.PossibleMoves(g.Board)
	legalMoves := make([]models.Position, 0, len(possibleMoves))

	for _, to := range possibleMoves {
		toSpot := g.Board.SpotAt(to)
		if toSpot == nil {
			continue
		}

		resolved, err := g.resolveMove(NewMove(position, to))
		if err != nil {
			continue
		}

		if resolved.CapturedPiece != nil &&
			resolved.CapturedPiece.Color() == resolved.MovingPiece.Color() {
			continue
		}

		if g.moveKeepsKingSafe(resolved) {
			legalMoves = append(legalMoves, to)
		}
	}

	g.legalMoves[position] = legalMoves

	return legalMoves, nil
}

func (g *Game) prepareNextTurn() error {
	g.legalMoves = make(map[models.Position][]models.Position)
	g.Turn = g.Turn.Flip()
	g.SquaresAttackedByWhite = g.CalculateAttackedSquares(models.White)
	g.SquaresAttackedByBlack = g.CalculateAttackedSquares(models.Black)

	var err error
	g.legalMoves, err = g.CalculateAllLegalMoves()
	if err != nil {
		return err
	}
	return nil
}

func (g *Game) capturePiece(piece models.Piece, capturedBy models.Color) {
	switch capturedBy {
	case models.White:
		g.CapturedByWhite = append(g.CapturedByWhite, piece)
		g.BlackPieces = removePiece(g.BlackPieces, piece)
	case models.Black:
		g.CapturedByBlack = append(g.CapturedByBlack, piece)
		g.WhitePieces = removePiece(g.WhitePieces, piece)
	}
}

func removePiece(allPieces []models.Piece, pieceToRemove models.Piece) []models.Piece {
	for i, piece := range allPieces {
		if piece == pieceToRemove {
			return append(allPieces[:i], allPieces[i+1:]...)
		}
	}
	return allPieces
}

func (g *Game) kingPosition(color models.Color) models.Position {
	if color == models.White {
		return g.WhiteKingPosition
	}

	return g.BlackKingPosition
}
