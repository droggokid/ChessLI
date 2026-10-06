package game

import (
	"time"

	"ChessLI/internal/identity"
)

type Outcome string

const (
	OutcomeNone     Outcome = ""
	OutcomeUnknown  Outcome = "unknown"
	OutcomeWhiteWin Outcome = "white_win"
	OutcomeBlackWin Outcome = "black_win"
	OutcomeDraw     Outcome = "draw"
)

type Termination string

const (
	TerminationNone                 Termination = ""
	TerminationCheckmate            Termination = "checkmate"
	TerminationStalemate            Termination = "stalemate"
	TerminationResignation          Termination = "resignation"
	TerminationTimeout              Termination = "timeout"
	TerminationDrawAgreement        Termination = "draw_agreement"
	TerminationThreefoldRepetition  Termination = "threefold_repetition"
	TerminationFivefoldRepetition   Termination = "fivefold_repetition"
	TerminationFiftyMoveRule        Termination = "fifty_move_rule"
	TerminationSeventyFiveMoveRule  Termination = "seventy_five_move_rule"
	TerminationInsufficientMaterial Termination = "insufficient_material"
)

type CompletedGame struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Outcome        Outcome
	Termination    Termination
	FinalFEN       string
	CompletedAt    time.Time
}

type Summary struct {
	ID             identity.GameID
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	Initial        time.Duration
	Increment      time.Duration
	Outcome        Outcome
	Termination    Termination
	CompletedAt    time.Time
}

type Move struct {
	Ply            int
	UCI            string
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
}
