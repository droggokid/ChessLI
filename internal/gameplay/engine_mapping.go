package gameplay

import (
	domain "ChessLI/internal/game"
	"github.com/corentings/chess/v2"
)

func terminationReason(method chess.Method) domain.Termination {
	switch method {
	case chess.Checkmate:
		return domain.TerminationCheckmate
	case chess.Stalemate:
		return domain.TerminationStalemate
	case chess.Resignation:
		return domain.TerminationResignation
	case chess.DrawOffer:
		return domain.TerminationDrawAgreement
	case chess.ThreefoldRepetition:
		return domain.TerminationThreefoldRepetition
	case chess.FivefoldRepetition:
		return domain.TerminationFivefoldRepetition
	case chess.FiftyMoveRule:
		return domain.TerminationFiftyMoveRule
	case chess.SeventyFiveMoveRule:
		return domain.TerminationSeventyFiveMoveRule
	case chess.InsufficientMaterial:
		return domain.TerminationInsufficientMaterial
	default:
		return domain.TerminationNone
	}
}

func outcomeFromEngine(outcome chess.Outcome) domain.Outcome {
	switch outcome {
	case chess.WhiteWon:
		return domain.OutcomeWhiteWin
	case chess.BlackWon:
		return domain.OutcomeBlackWin
	case chess.Draw:
		return domain.OutcomeDraw
	case chess.UnknownOutcome:
		return domain.OutcomeUnknown
	default:
		return domain.OutcomeNone
	}
}
