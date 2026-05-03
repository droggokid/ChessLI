package model

type Rook struct {
	BasePiece Piece
}

func NewRook(color Color) Piece {
	return &Rook{
		BasePiece: NewPiece(color),
	}
}

func (rook *Rook) String() string {
	if rook == nil {
		return "rook"
	}
	return describePiece("rook", rook.BasePiece)
}

func (rook *Rook) LegalMoves() []Position {
	return nil
}

func (rook *Rook) Move() {
	return
}
