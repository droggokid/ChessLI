package gameplay

import (
	"reflect"
	"slices"
	"testing"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

func TestGameCompletedRecordRetainsMovesAndTimeControl(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
	g := newGame("game", "white", "black", time.Minute, 2*time.Second)
	g.now = func() time.Time { return now }
	g.clock.start(now)
	if _, ok := g.completedRecord(); ok {
		t.Fatal("unfinished game returned a completed record")
	}

	for i, move := range []struct {
		profile  identity.ProfileID
		text     string
		notation MoveNotation
		elapsed  time.Duration
	}{
		{"white", "e4", MoveNotationSAN, 5 * time.Second},
		{"black", "e7e5", MoveNotationUCI, 3 * time.Second},
	} {
		now = now.Add(move.elapsed)
		if _, err := g.move(MoveCommand{ProfileID: move.profile, Move: move.text, Notation: move.notation, ExpectedVersion: uint64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(4 * time.Second)
	state, err := g.resign(ResignCommand{ProfileID: "white"})
	if err != nil {
		t.Fatal(err)
	}
	want := completedGameRecord{
		ID: "game", WhiteProfileID: "white", BlackProfileID: "black",
		Initial: time.Minute, Increment: 2 * time.Second,
		Outcome: domain.OutcomeBlackWin, Termination: domain.TerminationResignation,
		FinalFEN:    "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
		CompletedAt: now,
		Moves: []recordedMove{
			{Ply: 1, UCI: "e2e4", WhiteRemaining: 57 * time.Second, BlackRemaining: time.Minute},
			{Ply: 2, UCI: "e7e5", WhiteRemaining: 57 * time.Second, BlackRemaining: 59 * time.Second},
		},
	}
	record, ok := g.completedRecord()
	if !ok || !reflect.DeepEqual(record, want) {
		t.Fatalf("completed record = %+v, %v; want %+v", record, ok, want)
	}
	if state.WhiteRemaining != 53*time.Second {
		t.Fatalf("resignation clock = %v, want 53s", state.WhiteRemaining)
	}

	record.Moves[0].UCI = "tampered"
	record.Moves[1].WhiteRemaining = 0
	record.ID = "tampered"
	now = now.Add(time.Hour)
	if _, err := g.resume("white"); err != nil {
		t.Fatal(err)
	}
	g.snapshot(now)
	again, ok := g.completedRecord()
	if !ok || !reflect.DeepEqual(again, want) {
		t.Fatalf("record changed after caller mutation or resume: %+v", again)
	}
}

func TestGameCompletedRecordTerminalPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fen         string
		moves       []string
		finish      string
		outcome     domain.Outcome
		termination domain.Termination
	}{
		{name: "checkmate", moves: []string{"f2f3", "e7e5", "g2g4", "d8h4"}, outcome: domain.OutcomeBlackWin, termination: domain.TerminationCheckmate},
		{name: "stalemate", fen: "k7/8/1QK5/8/8/8/8/8 w - - 0 1", moves: []string{"b6c7"}, outcome: domain.OutcomeDraw, termination: domain.TerminationStalemate},
		{name: "insufficient material", fen: "k7/8/2K5/8/8/8/7r/6B1 w - - 0 1", moves: []string{"g1h2"}, outcome: domain.OutcomeDraw, termination: domain.TerminationInsufficientMaterial},
		{name: "fivefold repetition", moves: slices.Repeat([]string{"b1c3", "b8c6", "c3b1", "c6b8"}, 4), outcome: domain.OutcomeDraw, termination: domain.TerminationFivefoldRepetition},
		{name: "seventy-five moves", fen: "k7/8/2K5/8/8/8/8/7R w - - 149 76", moves: []string{"h1g1"}, outcome: domain.OutcomeDraw, termination: domain.TerminationSeventyFiveMoveRule},
		{name: "resignation without moves", finish: "resign", outcome: domain.OutcomeBlackWin, termination: domain.TerminationResignation},
		{name: "agreed draw", finish: "draw", outcome: domain.OutcomeDraw, termination: domain.TerminationDrawAgreement},
		{name: "late timeout observation", finish: "timeout", outcome: domain.OutcomeBlackWin, termination: domain.TerminationTimeout},
		{name: "timeout against bare king", fen: "8/8/8/8/8/2k5/8/R2K4 w - - 0 1", finish: "timeout", outcome: domain.OutcomeDraw, termination: domain.TerminationTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)
			g := newGame("game", "white", "black", time.Minute, 2*time.Second)
			g.now = func() time.Time { return now }
			g.clock.start(now)
			if tt.fen != "" {
				position, err := chess.FEN(tt.fen)
				if err != nil {
					t.Fatal(err)
				}
				g.engine = chess.NewGame(position)
			}
			for i, move := range tt.moves {
				profile := identity.ProfileID("white")
				if i%2 == 1 {
					profile = "black"
				}
				if _, err := g.move(MoveCommand{ProfileID: profile, Move: move, Notation: MoveNotationUCI, ExpectedVersion: uint64(i)}); err != nil {
					t.Fatal(err)
				}
			}
			completedAt := now
			switch tt.finish {
			case "resign":
				if _, err := g.resign(ResignCommand{ProfileID: "white"}); err != nil {
					t.Fatal(err)
				}
			case "draw":
				offer, err := g.offerDraw(OfferDrawCommand{ProfileID: "white"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := g.acceptDraw(DrawOfferResponseCommand{ProfileID: "black", OfferID: offer.PendingDrawOffer.OfferID}); err != nil {
					t.Fatal(err)
				}
			case "timeout":
				completedAt = now.Add(time.Minute)
				now = now.Add(2 * time.Minute)
				state, err := g.move(MoveCommand{ProfileID: "white", Move: "e2e4", Notation: MoveNotationUCI})
				if err != nil || state.Outcome != tt.outcome {
					t.Fatalf("timeout move = %+v, %v", state, err)
				}
			}
			record, ok := g.completedRecord()
			if !ok || record.Outcome != tt.outcome || record.Termination != tt.termination || !record.CompletedAt.Equal(completedAt) || record.FinalFEN != g.snapshot(now).FEN || len(record.Moves) != len(tt.moves) {
				t.Fatalf("completed record = %+v, %v", record, ok)
			}
			for i, move := range record.Moves {
				wantWhite := time.Minute + time.Duration((i+2)/2)*2*time.Second
				wantBlack := time.Minute + time.Duration((i+1)/2)*2*time.Second
				if move.Ply != i+1 || move.UCI != tt.moves[i] || move.WhiteRemaining != wantWhite || move.BlackRemaining != wantBlack {
					t.Fatalf("move %d = %+v, want %s with clocks %v/%v", i+1, move, tt.moves[i], wantWhite, wantBlack)
				}
			}
			now = now.Add(time.Hour)
			if _, err := g.resume("black"); err != nil {
				t.Fatal(err)
			}
			if g.expire(now) {
				t.Fatal("finished game expired again")
			}
			again, _ := g.completedRecord()
			if !reflect.DeepEqual(again, record) {
				t.Fatalf("completion changed: %+v -> %+v", record, again)
			}
		})
	}
}
