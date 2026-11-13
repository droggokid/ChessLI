package gameplay

import (
	domain "ChessLI/internal/game"
	"testing"

	"github.com/corentings/chess/v2"
)

func TestTerminationReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method chess.Method
		want   domain.Termination
	}{
		{name: "none", method: chess.NoMethod, want: domain.TerminationNone},
		{name: "checkmate", method: chess.Checkmate, want: domain.TerminationCheckmate},
		{name: "stalemate", method: chess.Stalemate, want: domain.TerminationStalemate},
		{name: "resignation", method: chess.Resignation, want: domain.TerminationResignation},
		{name: "draw agreement", method: chess.DrawOffer, want: domain.TerminationDrawAgreement},
		{name: "threefold repetition", method: chess.ThreefoldRepetition, want: domain.TerminationThreefoldRepetition},
		{name: "fivefold repetition", method: chess.FivefoldRepetition, want: domain.TerminationFivefoldRepetition},
		{name: "fifty-move rule", method: chess.FiftyMoveRule, want: domain.TerminationFiftyMoveRule},
		{name: "seventy-five-move rule", method: chess.SeventyFiveMoveRule, want: domain.TerminationSeventyFiveMoveRule},
		{name: "insufficient material", method: chess.InsufficientMaterial, want: domain.TerminationInsufficientMaterial},
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
