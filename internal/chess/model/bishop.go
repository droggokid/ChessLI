package model

type Bishop struct {
	BasePiece Piece
}

func NewBishop(color Color) Piece {
	return &Bishop{
		BasePiece: NewPiece(color),
	}
}

func (bishop *Bishop) String() string {
	if bishop == nil {
		return "bishop"
	}
	return describePiece("bishop", bishop.BasePiece)
}

func (bishop *Bishop) LegalMoves() []Position {
	return nil
}

func (bishop *Bishop) Move() {
	return
}
