package rules

import (
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/moves"
	"errors"
)

type Rules struct {
	moveService moves.MoveService

	board models.BoardView
	turn  models.Color

	whitePieces []models.Piece
	blackPieces []models.Piece

	whiteKingPosition models.Position
	blackKingPosition models.Position

	lastMove        *models.Move
	legalMoves      map[models.Position][]models.Position
	attackedSquares map[models.Position]bool
}

func NewRules(
	moveService moves.MoveService,
	board models.BoardView,
	turn models.Color,
	whitePieces []models.Piece,
	blackPieces []models.Piece,
	whiteKingPosition models.Position,
	blackKingPosition models.Position,
	lastMove *models.Move,
) (Rules, error) {
	newRules := Rules{
		moveService:       moveService,
		board:             board,
		turn:              turn,
		whitePieces:       whitePieces,
		blackPieces:       blackPieces,
		whiteKingPosition: whiteKingPosition,
		blackKingPosition: blackKingPosition,
		lastMove:          lastMove,
		legalMoves:        make(map[models.Position][]models.Position),
		attackedSquares:   make(map[models.Position]bool),
	}

	var err error
	newRules.legalMoves, err = newRules.CalculateAllLegalMoves()
	if err != nil {
		return Rules{}, err
	}

	newRules.attackedSquares = newRules.CalculateAttackedSquares(turn.Flip())

	return newRules, nil
}

// CalculateAttackedSquares returns the set of squares controlled by the given color.
func (r *Rules) CalculateAttackedSquares(by models.Color) map[models.Position]bool {
	attackedSquares := make(map[models.Position]bool)

	for _, piece := range r.piecesFor(by) {
		spot := r.board.SpotAt(piece.Position())
		if spot == nil || spot.Piece != piece {
			continue
		}

		for _, pos := range piece.AttackedSquares(r.board) {
			attackedSquares[pos] = true
		}
	}

	return attackedSquares
}

func (r *Rules) piecesFor(color models.Color) []models.Piece {
	if color == models.White {
		return r.whitePieces
	}
	return r.blackPieces
}

// CalculateAllLegalMoves calculates legal moves for the current player.
func (r *Rules) CalculateAllLegalMoves() (map[models.Position][]models.Position, error) {
	legalMoves := make(map[models.Position][]models.Position)

	var err error
	if r.turn == models.White {
		for _, piece := range r.whitePieces {
			legalMoves[piece.Position()], err = r.LegalMovesFor(piece.Position())
			if err != nil {
				return nil, err
			}
		}
	} else {
		for _, piece := range r.blackPieces {
			legalMoves[piece.Position()], err = r.LegalMovesFor(piece.Position())
			if err != nil {
				return nil, err
			}
		}
	}

	return legalMoves, nil
}

// LegalMovesFor returns legal moves for the current player's piece at position.
func (r *Rules) LegalMovesFor(position models.Position) ([]models.Position, error) {
	if legalMoves, ok := r.legalMoves[position]; ok {
		return legalMoves, nil
	}

	fromSpot := r.board.SpotAt(position)
	if fromSpot == nil {
		return nil, errors.New("no legal moves found")
	}

	movingPiece := fromSpot.Piece
	if movingPiece == nil || movingPiece.Color() != r.turn {
		return nil, errors.New("no legal moves found")
	}

	possibleMoves := movingPiece.PossibleMoves(r.board, r.lastMove)
	legalMoves := make([]models.Position, 0, len(possibleMoves))

	for _, to := range possibleMoves {
		toSpot := r.board.SpotAt(to)
		if toSpot == nil {
			continue
		}

		resolved, err := r.moveService.ResolveMove(models.NewMove(position, to))
		if err != nil {
			continue
		}

		if resolved.CapturedPiece != nil &&
			resolved.CapturedPiece.Color() == resolved.MovingPiece.Color() {
			continue
		}

		if r.moveKeepsKingSafe(resolved) {
			legalMoves = append(legalMoves, to)
		}
	}

	r.legalMoves[position] = legalMoves

	return legalMoves, nil
}

func (r *Rules) IsDraw() bool {
	return r.InsufficientMaterial() || r.IsStalemate()
}

func (r *Rules) InsufficientMaterial() bool {
	whitePieces := nonKingPieces(r.whitePieces)
	blackPieces := nonKingPieces(r.blackPieces)

	if insufficientAgainstAnyMaterial(whitePieces) && insufficientAgainstAnyMaterial(blackPieces) {
		return true
	}

	if twoKnightsAgainstLoneKing(whitePieces, blackPieces) {
		return true
	}

	return false
}

func (r *Rules) IsStalemate() bool {
	if r.CurrentPlayerIsInCheck() {
		return false
	}

	for _, m := range r.legalMoves {
		if len(m) > 0 {
			return false
		}
	}

	return true
}

func (r *Rules) IsCheckmate() bool {
	if !r.CurrentPlayerIsInCheck() {
		return false
	}

	for _, m := range r.legalMoves {
		if len(m) > 0 {
			return false
		}
	}

	return true
}

func (r *Rules) CurrentPlayerIsInCheck() bool {
	kingPosition := r.kingPosition(r.turn)
	_, found := r.attackedSquares[kingPosition]
	return found
}

func (r *Rules) IsLegalMove(move models.Move) bool {
	legalMoves := r.legalMoves[move.From]
	for _, m := range legalMoves {
		if m == move.To {
			return true
		}
	}
	return false
}

func nonKingPieces(pieces []models.Piece) []models.Piece {
	nonKings := make([]models.Piece, 0, len(pieces))
	for _, piece := range pieces {
		if piece.Type() != models.King {
			nonKings = append(nonKings, piece)
		}
	}
	return nonKings
}

func insufficientAgainstAnyMaterial(pieces []models.Piece) bool {
	if len(pieces) == 0 {
		return true
	}

	return len(pieces) == 1 && isSingleMinorPiece(pieces[0])
}

func isSingleMinorPiece(piece models.Piece) bool {
	return piece.Type() == models.Bishop || piece.Type() == models.Knight
}

func twoKnightsAgainstLoneKing(whitePieces []models.Piece, blackPieces []models.Piece) bool {
	return hasOnlyTwoKnights(whitePieces) && len(blackPieces) == 0 ||
		hasOnlyTwoKnights(blackPieces) && len(whitePieces) == 0
}

func hasOnlyTwoKnights(pieces []models.Piece) bool {
	if len(pieces) != 2 {
		return false
	}

	return pieces[0].Type() == models.Knight && pieces[1].Type() == models.Knight
}

// moveKeepsKingSafe simulates a candidate move and reports whether the moving
// side's king is safe afterward. It always restores board and king-position
// state before returning.
func (r *Rules) moveKeepsKingSafe(resolved models.ResolvedMove) bool {
	oldKingPosition := r.kingPosition(resolved.MovingPiece.Color())

	r.moveService.ApplyMove(resolved)
	r.updateKingPosition(resolved)

	kingPosition := r.kingPosition(resolved.MovingPiece.Color())
	attacked := r.CalculateAttackedSquares(resolved.MovingPiece.Color().Flip())
	_, kingIsAttacked := attacked[kingPosition]

	r.moveService.RevertMove(resolved)
	r.restoreKingPosition(resolved, oldKingPosition)

	return !kingIsAttacked
}

func (r *Rules) kingPosition(color models.Color) models.Position {
	if color == models.White {
		return r.whiteKingPosition
	}

	return r.blackKingPosition
}

func (r *Rules) updateKingPosition(resolved models.ResolvedMove) {
	if resolved.MovingPiece.Type() != models.King {
		return
	}

	if resolved.MovingPiece.Color() == models.White {
		r.whiteKingPosition = resolved.ToSpot.Position
	} else {
		r.blackKingPosition = resolved.ToSpot.Position
	}
}

func (r *Rules) restoreKingPosition(resolved models.ResolvedMove, old models.Position) {
	if resolved.MovingPiece.Type() != models.King {
		return
	}

	if resolved.MovingPiece.Color() == models.White {
		r.whiteKingPosition = old
	} else {
		r.blackKingPosition = old
	}
}
