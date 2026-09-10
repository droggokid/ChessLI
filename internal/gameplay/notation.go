package gameplay

import "github.com/corentings/chess/v2"

type MoveNotation uint8

const (
	MoveNotationUCI MoveNotation = iota
	MoveNotationSAN
	MoveNotationLAN
)

func engineNotation(notation MoveNotation) (chess.Notation, error) {
	switch notation {
	case MoveNotationUCI:
		return chess.UCINotation{}, nil
	case MoveNotationSAN:
		return chess.AlgebraicNotation{}, nil
	case MoveNotationLAN:
		return chess.LongAlgebraicNotation{}, nil
	default:
		return nil, ErrUnsupportedNotation
	}
}
