// Package game owns players, turns, captures, move orchestration, and check safety.
package game

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"chessli/internal/chess/moves"
	"chessli/internal/chess/rules"
	"errors"
)

// Game contains the mutable state for one chess game.
type Game struct {
	// State
	moveService moves.MoveService

	State  GameState
	Result *GameResult

	Board *board.Board
	Turn  models.Color

	WhitePlayer Player
	BlackPlayer Player

	WhitePieces []models.Piece
	BlackPieces []models.Piece

	CapturedByWhite []models.Piece
	CapturedByBlack []models.Piece

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
	PromotedTo    models.PieceType
	WasPromotion  bool
	WasEnPassant  bool
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
	gameRules, err := game.prepareRules(nil)
	if err != nil {
		return Game{}, err
	}

	game.rules = gameRules

	game.updateState()

	return game, nil
}

func (g *Game) Move(move models.Move) error {
	resolved, err := g.VerifyMove(move)
	if err != nil {
		return err
	}

	if resolved.CapturedPiece != nil {
		g.capturePiece(resolved.CapturedPiece, resolved.MovingPiece.Color())
	}

	if isPromotionMove(resolved) {
		promotedPiece, err := newPromotionPiece(g.Turn, resolved.Move.To, *resolved.Move.Promotion)
		if err != nil {
			return err
		}

		g.moveService.ApplyMove(resolved)
		g.replacePromotedPawn(resolved.MovingPiece, promotedPiece)
		resolved.ToSpot.Piece = promotedPiece
	} else {
		g.moveService.ApplyMove(resolved)
	}

	g.updateKingPosition(resolved)

	g.recordMove(resolved)

	err = g.prepareNextTurn()
	if err != nil {
		return err
	}

	return nil
}

func (g *Game) VerifyMove(move models.Move) (models.ResolvedMove, error) {
	resolved, err := g.moveService.ResolveMove(move)
	if err != nil {
		return models.ResolvedMove{}, err
	}

	if resolved.MovingPiece.Color() != g.Turn {
		return models.ResolvedMove{}, errors.New("cannot move opponent's piece")
	}

	if !g.rules.IsLegalMove(move) {
		return models.ResolvedMove{}, errors.New("illegal move")
	}

	if isPromotionMove(resolved) && resolved.Move.Promotion == nil {
		return models.ResolvedMove{}, errors.New("cannot promote move")
	}

	if isPromotionMove(resolved) && !isPromotionType(*resolved.Move.Promotion) {
		return models.ResolvedMove{}, errors.New("invalid promotion type")
	}

	if !isPromotionMove(resolved) && resolved.Move.Promotion != nil {
		return models.ResolvedMove{}, errors.New("cannot promote move")
	}

	return resolved, nil
}

func (g *Game) prepareRules(lastMove *models.Move) (rules.Rules, error) {
	newRules, err := rules.NewRules(
		g.moveService,
		g.Board,
		g.Turn,
		g.WhitePieces,
		g.BlackPieces,
		g.WhiteKingPosition,
		g.BlackKingPosition,
		lastMove,
	)

	if err != nil {
		return rules.Rules{}, err
	}

	return newRules, nil
}

func newPromotionPiece(color models.Color, position models.Position, pieceType models.PieceType) (models.Piece, error) {
	var promotedPiece models.Piece

	switch pieceType {
	case models.Knight:
		promotedPiece = pieces.NewKnight(color, position)
	case models.Bishop:
		promotedPiece = pieces.NewBishop(color, position)
	case models.Rook:
		promotedPiece = pieces.NewRook(color, position)
	case models.Queen:
		promotedPiece = pieces.NewQueen(color, position)
	default:
		return nil, errors.New("invalid piece type")
	}

	return promotedPiece, nil
}

func (g *Game) replacePromotedPawn(pawn, promotedPiece models.Piece) {
	if pawn.Color() == models.White {
		g.WhitePieces = removePiece(g.WhitePieces, pawn)
		g.WhitePieces = append(g.WhitePieces, promotedPiece)
	} else {
		g.BlackPieces = removePiece(g.BlackPieces, pawn)
		g.BlackPieces = append(g.BlackPieces, promotedPiece)
	}
}

func (g *Game) LegalMovesFor(pos models.Position) ([]models.Position, error) {
	return g.rules.LegalMovesFor(pos)
}

func (g *Game) IsDraw() bool {
	return g.rules.IsDraw()
}

func (g *Game) IsStalemate() bool {
	return g.rules.IsStalemate()
}

func (g *Game) IsCheckmate() bool {
	return g.rules.IsCheckmate()
}

func (g *Game) CurrentPlayerIsInCheck() bool {
	return g.rules.CurrentPlayerIsInCheck()
}

func (g *Game) prepareNextTurn() error {
	g.Turn = g.Turn.Flip()

	var (
		err      error
		lastMove *models.Move
	)

	if len(g.MoveHistory) != 0 {
		lastMove = &g.MoveHistory[len(g.MoveHistory)-1].Move
	} else {
		lastMove = nil
	}

	g.rules, err = g.prepareRules(lastMove)
	if err != nil {
		return err
	}

	g.updateState()

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
		PromotedTo:    models.PieceType(""),
		WasPromotion:  isPromotionMove(resolvedMove),
		WasEnPassant:  resolvedMove.WasEnPassant,
	}

	if record.WasPromotion {
		record.PromotedTo = *resolvedMove.Move.Promotion
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

func isPromotionMove(resolved models.ResolvedMove) bool {
	return resolved.MovingPiece.Type() == models.Pawn &&
		(resolved.ToSpot.Position.Rank == models.Rank1 || resolved.ToSpot.Position.Rank == models.Rank8)
}

func isPromotionType(promotion models.PieceType) bool {
	return promotion == models.Queen ||
		promotion == models.Rook ||
		promotion == models.Bishop ||
		promotion == models.Knight
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

func (g *Game) updateState() {
	g.Result = NewGameResult()

	switch {
	case g.IsCheckmate():
		g.State = GameStateCheckmate
		g.handleWin()
	case g.IsDraw():
		g.State = GameStateDraw
		g.handleDraw()
	case g.CurrentPlayerIsInCheck():
		g.State = GameStateCheck
	default:
		g.State = GameStateActive
	}
}

func (g *Game) handleWin() {
	winnerColor := g.Turn.Flip()
	if winnerColor == models.White {
		g.Result.Winner = &g.WhitePlayer
	} else if winnerColor == models.Black {
		g.Result.Winner = &g.BlackPlayer
	}
}

func (g *Game) handleDraw() {
	g.Result.Draw = true
}
