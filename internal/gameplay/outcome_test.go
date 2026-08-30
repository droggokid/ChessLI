package gameplay

import (
	"testing"

	"github.com/corentings/chess/v2"
)

func TestTerminationReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method chess.Method
		want   TerminationReason
	}{
		{name: "none", method: chess.NoMethod, want: TerminationNone},
		{name: "checkmate", method: chess.Checkmate, want: TerminationCheckmate},
		{name: "stalemate", method: chess.Stalemate, want: TerminationStalemate},
		{name: "resignation", method: chess.Resignation, want: TerminationResignation},
		{name: "draw agreement", method: chess.DrawOffer, want: TerminationDrawAgreement},
		{name: "threefold repetition", method: chess.ThreefoldRepetition, want: TerminationThreefoldRepetition},
		{name: "fivefold repetition", method: chess.FivefoldRepetition, want: TerminationFivefoldRepetition},
		{name: "fifty-move rule", method: chess.FiftyMoveRule, want: TerminationFiftyMoveRule},
		{name: "seventy-five-move rule", method: chess.SeventyFiveMoveRule, want: TerminationSeventyFiveMoveRule},
		{name: "insufficient material", method: chess.InsufficientMaterial, want: TerminationInsufficientMaterial},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := terminationReason(tt.method); got != tt.want {
				t.Fatalf("terminationReason(%v) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
