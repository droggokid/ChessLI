package model

type Knight struct {
	BasePiece Piece
}

func NewKnight(color Color) Piece {
	return &Knight{
		BasePiece: NewPiece(color),
	}
}

func (knight *Knight) String() string {
	if knight == nil {
		return "knight"
	}
	return describePiece("knight", knight.BasePiece)
}

func (knight *Knight) LegalMoves() []Position {
	return nil
}

func (knight *Knight) Move() {
	return
}
