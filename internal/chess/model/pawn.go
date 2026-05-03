package model

type Pawn struct {
	BasePiece Piece
}

func NewPawn(color Color) Piece {
	return &Pawn{
		BasePiece: NewPiece(color),
	}
}

func (pawn *Pawn) String() string {
	if pawn == nil {
		return "pawn"
	}
	return describePiece("pawn", pawn.BasePiece)
}

func (pawn *Pawn) LegalMoves() []Position {
	return nil
}

func (pawn *Pawn) Move() {
	return
}
