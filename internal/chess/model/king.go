package model

type King struct {
	BasePiece Piece
}

func NewKing(color Color) Piece {
	return &King{
		BasePiece: NewPiece(color),
	}
}

func (king *King) String() string {
	if king == nil {
		return "king"
	}
	return describePiece("king", king.BasePiece)
}

func (king *King) LegalMoves() []Position {
	return nil
}

func (king *King) Move() {
	return
}
