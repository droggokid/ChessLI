package game

import (
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/board/pieces"
	"errors"
	"fmt"
)

// Move represents a requested board transition.
type Move struct {
	From models.Position
	To   models.Position
}

type resolvedMove struct {
	Move          Move
	FromSpot      *models.Spot
	ToSpot        *models.Spot
	MovingPiece   models.Piece
	CapturedPiece models.Piece
}

// MoveRecord stores stable move history data.
type MoveRecord struct {
	Move        Move
	MovingColor models.Color
	MovingPiece models.PieceType
	// CapturedPiece is empty when no piece was captured.
	CapturedPiece models.PieceType
	WasCapture    bool
}

// NewMove creates a move from one position to another.
func NewMove(from, to models.Position) Move {
	return Move{From: from, To: to}
}

func (m Move) String() string {
	return fmt.Sprintf("%s %s", m.From, m.To)
}

// Move applies a legal move for the current player and records it.
func (g *Game) Move(move Move) error {
	resolved, err := g.verifyMove(move)
	if err != nil {
		return err
	}

	if resolved.CapturedPiece != nil {
		g.capturePiece(resolved.CapturedPiece, resolved.MovingPiece.Color())
	}

	g.applyMove(resolved)

	g.recordMove(resolved)

	err = g.prepareNextTurn()
	if err != nil {
		return err
	}

	return nil
}

func (g *Game) verifyMove(move Move) (resolvedMove, error) {
	resolved, err := g.resolveMove(move)
	if err != nil {
		return resolvedMove{}, err
	}

	if resolved.MovingPiece.Color() != g.Turn {
		return resolvedMove{}, errors.New("cannot move opponent's piece")
	}

	if !g.isLegalMove(move) {
		return resolvedMove{}, errors.New("illegal move")
	}

	return resolved, nil
}

func (g *Game) resolveMove(move Move) (resolvedMove, error) {
	fromSpot := g.Board.SpotAt(move.From)
	toSpot := g.Board.SpotAt(move.To)

	if fromSpot == nil || toSpot == nil {
		return resolvedMove{}, errors.New("position outside board")
	}

	movingPiece := fromSpot.Piece
	if movingPiece == nil {
		return resolvedMove{}, errors.New("no piece at source position")
	}

	capturedPiece := toSpot.Piece

	return resolvedMove{
		Move:          move,
		FromSpot:      fromSpot,
		ToSpot:        toSpot,
		MovingPiece:   movingPiece,
		CapturedPiece: capturedPiece,
	}, nil
}

func (g *Game) recordMove(resolvedMove resolvedMove) {
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

// moveKeepsKingSafe simulates a candidate move and reports whether the moving
// side's king is safe afterward. It always restores board and king-position
// state before returning.
func (g *Game) moveKeepsKingSafe(resolvedMove resolvedMove) bool {
	movingPiece := resolvedMove.FromSpot.Piece

	var oldKingPosition models.Position
	if movingPiece.Color() == models.White {
		oldKingPosition = g.WhiteKingPosition
	} else {
		oldKingPosition = g.BlackKingPosition
	}

	g.applyMove(resolvedMove)

	simulatedKingPosition := g.kingPosition(movingPiece.Color())
	simulatedAttackedSquares := g.CalculateAttackedSquares(movingPiece.Color().Flip())

	_, kingIsAttacked := simulatedAttackedSquares[simulatedKingPosition]

	g.revertMove(resolvedMove, oldKingPosition)

	return !kingIsAttacked
}

func (g *Game) applyMove(resolved resolvedMove) {
	resolved.MovingPiece.MoveTo(resolved.ToSpot.Position)

	resolved.ToSpot.Piece = resolved.MovingPiece
	resolved.FromSpot.Piece = nil

	if _, ok := resolved.MovingPiece.(*pieces.King); ok {
		if resolved.MovingPiece.Color() == models.White {
			g.WhiteKingPosition = resolved.ToSpot.Position
		} else {
			g.BlackKingPosition = resolved.ToSpot.Position
		}
	}
}

func (g *Game) revertMove(resolvedMove resolvedMove, oldKingPosition models.Position) {
	resolvedMove.MovingPiece.MoveTo(resolvedMove.FromSpot.Position)

	resolvedMove.ToSpot.Piece = resolvedMove.CapturedPiece
	resolvedMove.FromSpot.Piece = resolvedMove.MovingPiece

	if _, ok := resolvedMove.MovingPiece.(*pieces.King); ok {
		if resolvedMove.MovingPiece.Color() == models.White {
			g.WhiteKingPosition = oldKingPosition
		} else {
			g.BlackKingPosition = oldKingPosition
		}
	}
}

func (g *Game) isLegalMove(move Move) bool {
	legalMoves := g.legalMoves[move.From]
	for _, m := range legalMoves {
		if m == move.To {
			return true
		}
	}
	return false
}
