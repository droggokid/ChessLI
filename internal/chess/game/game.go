// Package game owns players, turns, captures, move orchestration, and check safety.
package game

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"errors"
)

// Game contains the mutable state for one chess game.
type Game struct {
	Board *board.Board
	Turn models.Color

	WhitePlayer Player
	BlackPlayer Player

	// WhitePieces tracks uncaptured white pieces.
	WhitePieces []models.Piece
	BlackPieces []models.Piece

	// CapturedByWhite tracks pieces captured by white.
	CapturedByWhite []models.Piece
	CapturedByBlack []models.Piece

	// SquaresAttackedByWhite caches squares currently controlled by white.
	SquaresAttackedByWhite map[models.Position]bool
	SquaresAttackedByBlack map[models.Position]bool

	// WhiteKingPosition tracks the white king without scanning the board.
	WhiteKingPosition models.Position
	BlackKingPosition models.Position

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
		legalMoves:        make(map[models.Position][]models.Position),
	}
	game.SquaresAttackedByWhite = game.CalculateAttackedSquares(models.White)
	game.SquaresAttackedByBlack = game.CalculateAttackedSquares(models.Black)

	game.legalMoves = game.CalculateAllLegalMoves()

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

// CalculateAllLegalMoves returns legal moves for the current player.
func (g *Game) CalculateAllLegalMoves() map[models.Position][]models.Position {
	legalMoves := make(map[models.Position][]models.Position)

	if g.Turn == models.White {
		for _, piece := range g.WhitePieces {
			legalMoves[piece.Position()] = g.LegalMovesFor(piece.Position())
		}
	} else {
		for _, piece := range g.BlackPieces {
			legalMoves[piece.Position()] = g.LegalMovesFor(piece.Position())
		}
	}

	return legalMoves
}

// LegalMovesFor returns legal moves for the current player's piece at position.
func (g *Game) LegalMovesFor(position models.Position) []models.Position {
	if moves, ok := g.legalMoves[position]; ok {
		return moves
	}

	fromSpot := g.Board.SpotAt(position)
	if fromSpot == nil {
		return nil
	}

	piece := fromSpot.Piece
	if piece == nil || piece.Color() != g.Turn {
		return nil
	}

	possibleMoves := piece.PossibleMoves(g.Board)
	legalMoves := make([]models.Position, 0, len(possibleMoves))

	for _, move := range possibleMoves {
		toSpot := g.Board.SpotAt(move)
		if toSpot == nil {
			continue
		}

		if g.moveKeepsKingSafe(fromSpot, toSpot) {
			legalMoves = append(legalMoves, move)
		}
	}

	g.legalMoves[position] = legalMoves
	return legalMoves
}

func (g *Game) prepareNextTurn() {
	g.legalMoves = make(map[models.Position][]models.Position)
	g.Turn = g.Turn.Flip()
	g.SquaresAttackedByWhite = g.CalculateAttackedSquares(models.White)
	g.SquaresAttackedByBlack = g.CalculateAttackedSquares(models.Black)
	g.legalMoves = g.CalculateAllLegalMoves()
}

// Move moves the current player's piece from one square to another.
//
// The move must already be present in the legal move cache for the current turn.
func (g *Game) Move(from models.Position, to models.Position) error {
	fromSpot := g.Board.SpotAt(from)
	toSpot := g.Board.SpotAt(to)

	if fromSpot == nil || toSpot == nil {
		return errors.New("position outside board")
	}

	movingPiece := fromSpot.Piece
	if movingPiece == nil {
		return errors.New("no piece at source position")
	}

	if movingPiece.Color() != g.Turn {
		return errors.New("cannot move opponent's piece")
	}

	if !g.containsMove(from, to) {
		return errors.New("illegal move")
	}

	capturedPiece := toSpot.Piece
	if capturedPiece != nil {
		if capturedPiece.Color() == movingPiece.Color() {
			return errors.New("cannot capture own piece")
		}

		g.capturePiece(capturedPiece, movingPiece.Color())
	}

	g.applyMove(fromSpot, toSpot, movingPiece)

	g.prepareNextTurn()

	return nil
}

// moveKeepsKingSafe simulates a candidate move and reports whether the moving
// side's king is safe afterward. It always restores board and king-position
// state before returning.
func (g *Game) moveKeepsKingSafe(fromSpot, toSpot *models.Spot) bool {
	movingPiece := fromSpot.Piece
	capturedPiece := toSpot.Piece

	var oldKingPosition models.Position
	if movingPiece.Color() == models.White {
		oldKingPosition = g.WhiteKingPosition
	} else {
		oldKingPosition = g.BlackKingPosition
	}

	g.applyMove(fromSpot, toSpot, movingPiece)

	simulatedKingPosition := g.kingPosition(movingPiece.Color())
	simulatedAttackedSquares := g.CalculateAttackedSquares(movingPiece.Color().Flip())

	_, kingIsAttacked := simulatedAttackedSquares[simulatedKingPosition]

	g.revertMove(fromSpot, toSpot, movingPiece, capturedPiece, oldKingPosition)

	return !kingIsAttacked
}

// applyMove mutates board state and the moving piece's stored position.
func (g *Game) applyMove(fromSpot, toSpot *models.Spot, movingPiece models.Piece) {
	movingPiece.MoveTo(toSpot.Position)

	toSpot.Piece = movingPiece
	fromSpot.Piece = nil

	if _, ok := movingPiece.(*pieces.King); ok {
		if movingPiece.Color() == models.White {
			g.WhiteKingPosition = toSpot.Position
		} else {
			g.BlackKingPosition = toSpot.Position
		}
	}
}

// revertMove undoes applyMove for move simulation.
func (g *Game) revertMove(
	fromSpot, toSpot *models.Spot,
	movingPiece, capturedPiece models.Piece,
	oldKingPosition models.Position,
) {
	movingPiece.MoveTo(fromSpot.Position)

	toSpot.Piece = capturedPiece
	fromSpot.Piece = movingPiece

	if _, ok := movingPiece.(*pieces.King); ok {
		if movingPiece.Color() == models.White {
			g.WhiteKingPosition = oldKingPosition
		} else {
			g.BlackKingPosition = oldKingPosition
		}
	}
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

func (g *Game) containsMove(from models.Position, to models.Position) bool {
	moves := g.legalMoves[from]
	for _, move := range moves {
		if move == to {
			return true
		}
	}
	return false
}

func (g *Game) kingPosition(color models.Color) models.Position {
	if color == models.White {
		return g.WhiteKingPosition
	}

	return g.BlackKingPosition
}
