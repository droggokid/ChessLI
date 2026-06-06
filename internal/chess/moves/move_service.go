package moves

import (
	"chessli/internal/chess/board/models"
	"errors"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=move_service.go -destination=move_service_mock.go -package=moves
type MoveService interface {
	ResolveMove(move models.Move) (models.ResolvedMove, error)
	ApplyMove(move models.ResolvedMove)
	RevertMove(move models.ResolvedMove)
}

type MoveServiceImpl struct {
	board models.BoardView
}

func NewMoveService(board models.BoardView) *MoveServiceImpl {
	return &MoveServiceImpl{board: board}
}

func (s *MoveServiceImpl) ResolveMove(move models.Move) (models.ResolvedMove, error) {
	fromSpot := s.board.SpotAt(move.From)
	toSpot := s.board.SpotAt(move.To)

	if fromSpot == nil || toSpot == nil {
		return models.ResolvedMove{}, errors.New("position outside board")
	}

	movingPiece := fromSpot.Piece
	if movingPiece == nil {
		return models.ResolvedMove{}, errors.New("no piece at source position")
	}

	capturedSpot := toSpot
	capturedPiece := toSpot.Piece
	wasEnPassant := false

	if movingPiece.Type() == models.Pawn &&
		toSpot.Piece == nil &&
		move.From.File != move.To.File {

		capturedPosition := models.NewPosition(move.From.Rank, move.To.File)
		adjacentSpot := s.board.SpotAt(capturedPosition)

		if adjacentSpot != nil &&
			adjacentSpot.Piece != nil &&
			adjacentSpot.Piece.Type() == models.Pawn &&
			adjacentSpot.Piece.Color() != movingPiece.Color() {

			capturedSpot = adjacentSpot
			capturedPiece = adjacentSpot.Piece
			wasEnPassant = true
		}
	}

	return models.ResolvedMove{
		Move:          move,
		FromSpot:      fromSpot,
		ToSpot:        toSpot,
		MovingPiece:   movingPiece,
		CapturedPiece: capturedPiece,
		CapturedSpot:  capturedSpot,
		WasEnPassant:  wasEnPassant,
	}, nil
}

func (s *MoveServiceImpl) ApplyMove(resolved models.ResolvedMove) {
	resolved.MovingPiece.MoveTo(resolved.ToSpot.Position)

	resolved.ToSpot.Piece = resolved.MovingPiece
	resolved.FromSpot.Piece = nil

	if resolved.WasEnPassant {
		resolved.CapturedSpot.Piece = nil
	}
}

func (s *MoveServiceImpl) RevertMove(resolvedMove models.ResolvedMove) {
	resolvedMove.MovingPiece.MoveTo(resolvedMove.FromSpot.Position)
	resolvedMove.FromSpot.Piece = resolvedMove.MovingPiece

	if resolvedMove.WasEnPassant {
		resolvedMove.ToSpot.Piece = nil
		resolvedMove.CapturedSpot.Piece = resolvedMove.CapturedPiece
	} else {
		resolvedMove.ToSpot.Piece = resolvedMove.CapturedPiece
	}
}
