package game

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
