package gameplay

import (
	"log/slog"
	"time"

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

func (g *game) syncOutcomeFromEngineLocked() {
	g.outcome = outcomeFromEngine(g.engine.Outcome())
	g.termination = terminationReason(g.engine.Method())
}

func (g *game) expireLocked(now time.Time) bool {
	active := g.engine.Position().Turn()

	loser, expired := g.clock.expire(now, active)
	if !expired {
		return false
	}

	winner := loser.Other()
	if !g.hasMatingMaterialLocked(winner) {
		g.outcome = domain.OutcomeDraw
	} else if loser == chess.White {
		g.outcome = domain.OutcomeBlackWin
	} else {
		g.outcome = domain.OutcomeWhiteWin
	}

	g.termination = domain.TerminationTimeout
	g.version++
	g.drawOffers.pending = nil
	g.logFinishedLocked()

	return true
}

// Log the transition itself so repeated snapshots and broadcasts cannot count it twice.
func (g *game) logFinishedLocked() {
	slog.Info("game finished", "event", "game.finished", "game_id", g.id,
		"outcome", g.outcome, "termination", g.termination, "version", g.version)
}

// hasMatingMaterialLocked reports the minimum FIDE requirement for a timeout
// win: the player who still has time must have some piece capable of
// participating in checkmate. A bare king can never give checkmate.
func (g *game) hasMatingMaterialLocked(color chess.Color) bool {
	for _, piece := range g.engine.Position().Board().SquareMap() {
		if piece.Color() == color && piece.Type() != chess.King {
			return true
		}
	}

	return false
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
