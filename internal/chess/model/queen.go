package model

type Queen struct {
	BasePiece Piece
}

func NewQueen(color Color) Piece {
	return &Queen{
		BasePiece: NewPiece(color),
	}
}

func (queen *Queen) String() string {
	if queen == nil {
		return "queen"
	}
	return describePiece("queen", queen.BasePiece)
}

func (queen *Queen) LegalMoves() []Position {
	return nil
}

func (queen *Queen) Move() {
	return
}
