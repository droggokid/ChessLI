package websocket

import (
	"reflect"
	"testing"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

func TestMapTimeControlPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset        protocol.TimeControlPreset
		wantInitial   time.Duration
		wantIncrement time.Duration
		wantErr       bool
	}{
		{preset: protocol.TimeControlBullet1Plus0, wantInitial: time.Minute},
		{preset: protocol.TimeControlBullet1Plus1, wantInitial: time.Minute, wantIncrement: time.Second},
		{preset: protocol.TimeControlBullet2Plus1, wantInitial: 2 * time.Minute, wantIncrement: time.Second},
		{preset: protocol.TimeControlBlitz3Plus0, wantInitial: 3 * time.Minute},
		{preset: protocol.TimeControlBlitz3Plus2, wantInitial: 3 * time.Minute, wantIncrement: 2 * time.Second},
		{preset: protocol.TimeControlBlitz5Plus0, wantInitial: 5 * time.Minute},
		{preset: protocol.TimeControlRapid10Plus0, wantInitial: 10 * time.Minute},
		{preset: protocol.TimeControlRapid10Plus5, wantInitial: 10 * time.Minute, wantIncrement: 5 * time.Second},
		{preset: protocol.TimeControlRapid15Plus10, wantInitial: 15 * time.Minute, wantIncrement: 10 * time.Second},
		{preset: protocol.TimeControlClassical30Plus0, wantInitial: 30 * time.Minute},
		{preset: protocol.TimeControlPreset("custom"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			t.Parallel()

			initial, increment, err := mapTimeControlPreset(tt.preset)
			if (err != nil) != tt.wantErr {
				t.Fatalf("mapTimeControlPreset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if initial != tt.wantInitial || increment != tt.wantIncrement {
				t.Fatalf("mapTimeControlPreset() = (%v, %v), want (%v, %v)", initial, increment, tt.wantInitial, tt.wantIncrement)
			}
		})
	}
}

func TestProtocolToGameplayMappings(t *testing.T) {
	t.Parallel()

	t.Run("move notation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input   protocol.MoveNotation
			want    gameplay.MoveNotation
			wantErr bool
		}{
			{input: "", want: gameplay.MoveNotationUCI},
			{input: protocol.MoveNotationUCI, want: gameplay.MoveNotationUCI},
			{input: protocol.MoveNotationSAN, want: gameplay.MoveNotationSAN},
			{input: protocol.MoveNotationLAN, want: gameplay.MoveNotationLAN},
			{input: "pgn", wantErr: true},
		}

		for _, tt := range tests {
			got, err := mapMoveNotation(tt.input)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("mapMoveNotation(%q) = (%v, %v), want (%v, wantErr=%v)", tt.input, got, err, tt.want, tt.wantErr)
			}
		}
	})

	t.Run("color preference", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input   protocol.ColorPreference
			want    gameplay.ColorPreference
			wantErr bool
		}{
			{input: "", want: gameplay.ColorRandom},
			{input: protocol.ColorPreferenceRandom, want: gameplay.ColorRandom},
			{input: protocol.ColorPreferenceWhite, want: gameplay.ColorWhite},
			{input: protocol.ColorPreferenceBlack, want: gameplay.ColorBlack},
			{input: "green", wantErr: true},
		}

		for _, tt := range tests {
			got, err := mapColorPreference(tt.input)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("mapColorPreference(%q) = (%v, %v), want (%v, wantErr=%v)", tt.input, got, err, tt.want, tt.wantErr)
			}
		}
	})
}

func TestMapColorFromServerRejectsInvalidColor(t *testing.T) {
	t.Parallel()

	if _, err := mapColorFromServer(chess.NoColor); err == nil {
		t.Fatal("mapColorFromServer(NoColor) error = nil, want error")
	}
}

func TestMapOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		outcome     chess.Outcome
		termination gameplay.TerminationReason
		want        *protocol.GameOutcome
	}{
		{name: "in progress", outcome: chess.NoOutcome},
		{name: "unknown", outcome: chess.UnknownOutcome},
		{name: "white checkmate", outcome: chess.WhiteWon, termination: gameplay.TerminationCheckmate, want: &protocol.GameOutcome{Result: protocol.ResultWhiteWin, Reason: protocol.GameOverCheckmate}},
		{name: "black resignation", outcome: chess.BlackWon, termination: gameplay.TerminationResignation, want: &protocol.GameOutcome{Result: protocol.ResultBlackWin, Reason: protocol.GameOverResignation}},
		{name: "timeout", outcome: chess.WhiteWon, termination: gameplay.TerminationTimeout, want: &protocol.GameOutcome{Result: protocol.ResultWhiteWin, Reason: protocol.GameOverTimeout}},
		{name: "draw agreement", outcome: chess.Draw, termination: gameplay.TerminationDrawAgreement, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverAgreement}},
		{name: "stalemate", outcome: chess.Draw, termination: gameplay.TerminationStalemate, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverStalemate}},
		{name: "threefold repetition", outcome: chess.Draw, termination: gameplay.TerminationThreefoldRepetition, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverThreefoldRepetition}},
		{name: "fivefold repetition", outcome: chess.Draw, termination: gameplay.TerminationFivefoldRepetition, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverFivefoldRepetition}},
		{name: "fifty-move rule", outcome: chess.Draw, termination: gameplay.TerminationFiftyMoveRule, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverFiftyMoveRule}},
		{name: "seventy-five move", outcome: chess.Draw, termination: gameplay.TerminationSeventyFiveMoveRule, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverSeventyFiveMoveRule}},
		{name: "insufficient material", outcome: chess.Draw, termination: gameplay.TerminationInsufficientMaterial, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverInsufficientMaterial}},
		{name: "missing termination", outcome: chess.Draw, termination: gameplay.TerminationNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := mapOutcome(tt.outcome, tt.termination); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mapOutcome() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGameStatePayload(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil, NewGameSessions())
	state := gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		Version:        3,
		WhiteProfileID: "white",
		LastMoveSAN:    "Qh4#",
		Outcome:        chess.BlackWon,
		Termination:    gameplay.TerminationCheckmate,
		PendingDrawOffer: &gameplay.DrawOffer{
			OfferID:   "offer",
			OfferedBy: "white",
		},
	}

	got := handler.gameStatePayload(state)
	if got.GameID != state.GameID || got.FEN != state.FEN || got.Version != state.Version || got.LastMove != state.LastMoveSAN {
		t.Fatalf("gameStatePayload() = %+v, want snapshot values %+v", got, state)
	}
	if got.Status != protocol.GameStatusFinished {
		t.Fatalf("gameStatePayload() status = %q, want %q", got.Status, protocol.GameStatusFinished)
	}
	wantOutcome := &protocol.GameOutcome{Result: protocol.ResultBlackWin, Reason: protocol.GameOverCheckmate}
	if !reflect.DeepEqual(got.Outcome, wantOutcome) {
		t.Fatalf("gameStatePayload() outcome = %+v, want %+v", got.Outcome, wantOutcome)
	}
	if got.PendingDrawOffer == nil || got.PendingDrawOffer.OfferID != "offer" || got.PendingDrawOffer.OfferedBy != protocol.ColorWhite {
		t.Fatalf("gameStatePayload() draw offer = %+v, want white offer", got.PendingDrawOffer)
	}
}

func TestPlayerStateOmitsEmptySeat(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil, NewGameSessions())
	if got := handler.playerState("game", "", protocol.ColorBlack, 10*time.Minute); got != nil {
		t.Fatalf("playerState() = %+v, want nil for empty seat", got)
	}
}
