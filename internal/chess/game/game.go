package game

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"errors"
)

type Game struct {
	Board *board.Board
	Turn  models.Color

	WhitePlayer Player
	BlackPlayer Player

	WhitePieces []models.Piece
	BlackPieces []models.Piece

	CapturedByWhite []models.Piece
	CapturedByBlack []models.Piece

	legalMoves map[models.Position][]models.Position
}

func NewGame(player1 string, player2 string) Game {
	gameBoard := board.NewBoard()
	game := Game{
		Board:           gameBoard,
		Turn:            models.White,
		WhitePlayer:     NewPlayer(player1, models.White),
		BlackPlayer:     NewPlayer(player2, models.Black),
		WhitePieces:     gameBoard.WhiteStarterPieces(),
		BlackPieces:     gameBoard.BlackStarterPieces(),
		CapturedByWhite: make([]models.Piece, 0, 16),
		CapturedByBlack: make([]models.Piece, 0, 16),
		legalMoves:      make(map[models.Position][]models.Position),
	}
	game.CalculateAllLegalMoves()
	return game
}

func (g *Game) CalculateAllLegalMoves() {
	if g.Turn == models.White {
		for _, piece := range g.WhitePieces {
			g.legalMoves[piece.Position()] = g.LegalMovesFor(piece.Position())
		}
	} else {
		for _, piece := range g.BlackPieces {
			g.legalMoves[piece.Position()] = g.LegalMovesFor(piece.Position())
		}
	}
}

func (g *Game) LegalMovesFor(position models.Position) []models.Position {
	if moves, ok := g.legalMoves[position]; ok {
		return moves
	}

	spot := g.Board.SpotAt(position)
	if spot == nil {
		return nil
	}

	piece := spot.Piece
	if piece == nil || piece.Color() != g.Turn {
		return nil
	}

	moves := piece.LegalMoves(position, g.Board)
	g.legalMoves[position] = moves
	return moves
}

func (g *Game) prepareNextTurn() {
	g.legalMoves = make(map[models.Position][]models.Position)
	g.Turn = g.Turn.Flip()
	g.CalculateAllLegalMoves()
}

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

	movingPiece.MoveTo(toSpot.Position)

	toSpot.Piece = movingPiece
	fromSpot.Piece = nil

	g.prepareNextTurn()

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

func (g *Game) containsMove(from models.Position, to models.Position) bool {
	moves := g.legalMoves[from]
	for _, move := range moves {
		if move == to {
			return true
		}
	}
	return false
}
