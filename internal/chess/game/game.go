// Package game owns players, turns, captures, move orchestration, and check safety.
package game

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/moves"
	"chessli/internal/chess/rules"
	"errors"
)

// Game contains the mutable state for one chess game.
type Game struct {
	// State
	moveService moves.MoveService
	Board       *board.Board
	Turn        models.Color

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

	// Per-turn
	rules rules.Rules
}

// MoveRecord stores stable move history data.
type MoveRecord struct {
	Move        models.Move
	MovingColor models.Color
	MovingPiece models.PieceType
	// CapturedPiece is empty when no piece was captured.
	CapturedPiece models.PieceType
	WasCapture    bool
}

// NewGame creates a standard chess game with two named players.
func NewGame(player1 string, player2 string) (Game, error) {
	gameBoard := board.NewBoard()
	moveService := moves.NewMoveService(gameBoard)

	game := Game{
		moveService:       moveService,
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
	}
	gameRules, err := game.prepareRules()
	if err != nil {
		return Game{}, err
	}

	game.rules = gameRules

	game.SquaresAttackedByWhite = gameRules.CalculateAttackedSquares(models.White)
	game.SquaresAttackedByBlack = gameRules.CalculateAttackedSquares(models.Black)

	return game, nil
}

func (g *Game) Move(move models.Move) error {
	resolved, err := g.VerifyMove(move, g.Turn)
	if err != nil {
		return err
	}

	if resolved.CapturedPiece != nil {
		g.capturePiece(resolved.CapturedPiece, resolved.MovingPiece.Color())
	}

	g.moveService.ApplyMove(resolved)
	g.updateKingPosition(resolved)

	g.recordMove(resolved)

	err = g.prepareNextTurn()
	if err != nil {
		return err
	}

	return nil
}

func (g *Game) VerifyMove(move models.Move, turn models.Color) (models.ResolvedMove, error) {
	resolved, err := g.moveService.ResolveMove(move)
	if err != nil {
		return models.ResolvedMove{}, err
	}

	if resolved.MovingPiece.Color() != turn {
		return models.ResolvedMove{}, errors.New("cannot move opponent's piece")
	}

	if !g.rules.IsLegalMove(move) {
		return models.ResolvedMove{}, errors.New("illegal move")
	}

	return resolved, nil
}

func (g *Game) prepareRules() (rules.Rules, error) {
	newRules, err := rules.NewRules(
		g.moveService,
		g.Board,
		g.Turn,
		g.WhitePieces,
		g.BlackPieces,
		g.WhiteKingPosition,
		g.BlackKingPosition,
	)

	if err != nil {
		return rules.Rules{}, err
	}

	return newRules, nil
}

func (g *Game) LegalMovesFor(pos models.Position) ([]models.Position, error) {
	return g.rules.LegalMovesFor(pos)
}

func (g *Game) prepareNextTurn() error {
	g.Turn = g.Turn.Flip()

	var err error
	g.rules, err = g.prepareRules()
	if err != nil {
		return err
	}

	g.SquaresAttackedByWhite = g.rules.CalculateAttackedSquares(models.White)
	g.SquaresAttackedByBlack = g.rules.CalculateAttackedSquares(models.Black)

	return nil
}

func (g *Game) recordMove(resolvedMove models.ResolvedMove) {
	capturedPiece := models.PieceType("")
	wasCaptured := resolvedMove.CapturedPiece != nil

	if wasCaptured {
		capturedPiece = resolvedMove.CapturedPiece.Type()
	}

	record := MoveRecord{
		Move:          resolvedMove.Move,
		MovingColor:   resolvedMove.MovingPiece.Color(),
		MovingPiece:   resolvedMove.MovingPiece.Type(),
		CapturedPiece: capturedPiece,
		WasCapture:    wasCaptured,
	}

	g.MoveHistory = append(g.MoveHistory, record)
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

func (g *Game) updateKingPosition(resolved models.ResolvedMove) {
	if resolved.MovingPiece.Type() != models.King {
		return
	}

	if resolved.MovingPiece.Color() == models.White {
		g.WhiteKingPosition = resolved.ToSpot.Position
	} else {
		g.BlackKingPosition = resolved.ToSpot.Position
	}
}
