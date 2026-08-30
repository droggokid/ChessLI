package gameplay

import (
	"time"

	"github.com/corentings/chess/v2"
)

type TerminationReason uint8

const (
	TerminationNone TerminationReason = iota
	TerminationCheckmate
	TerminationStalemate
	TerminationResignation
	TerminationTimeout
	TerminationDrawAgreement
	TerminationThreefoldRepetition
	TerminationFivefoldRepetition
	TerminationFiftyMoveRule
	TerminationSeventyFiveMoveRule
	TerminationInsufficientMaterial
)

func terminationReason(method chess.Method) TerminationReason {
	switch method {
	case chess.Checkmate:
		return TerminationCheckmate
	case chess.Stalemate:
		return TerminationStalemate
	case chess.Resignation:
		return TerminationResignation
	case chess.DrawOffer:
		return TerminationDrawAgreement
	case chess.ThreefoldRepetition:
		return TerminationThreefoldRepetition
	case chess.FivefoldRepetition:
		return TerminationFivefoldRepetition
	case chess.FiftyMoveRule:
		return TerminationFiftyMoveRule
	case chess.SeventyFiveMoveRule:
		return TerminationSeventyFiveMoveRule
	case chess.InsufficientMaterial:
		return TerminationInsufficientMaterial
	default:
		return TerminationNone
	}
}

func (g *Game) syncOutcomeFromEngineLocked() {
	g.outcome = g.engine.Outcome()
	g.termination = terminationReason(g.engine.Method())
}

func (g *Game) expireLocked(now time.Time) bool {
	active := g.engine.Position().Turn()

	loser, expired := g.clock.expire(now, active)
	if !expired {
		return false
	}

	switch loser {
	case chess.White:
		g.outcome = chess.BlackWon
	case chess.Black:
		g.outcome = chess.WhiteWon
	default:
		return false
	}

	g.termination = TerminationTimeout
	g.version++

	return true
}
