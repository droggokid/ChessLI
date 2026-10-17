package gameplay

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

func TestGameLogsCompletionOnce(t *testing.T) {
	for _, ending := range []domain.Termination{
		domain.TerminationCheckmate, domain.TerminationResignation,
		domain.TerminationDrawAgreement, domain.TerminationTimeout,
	} {
		t.Run(string(ending), func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			g := newReadyGame()
			switch ending {
			case domain.TerminationCheckmate:
				for i, move := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
					player := identity.ProfileID("white")
					if i%2 == 1 {
						player = "black"
					}
					if _, err := g.move(MoveCommand{ProfileID: player, Move: move, ExpectedVersion: uint64(i)}); err != nil {
						t.Fatal(err)
					}
				}
			case domain.TerminationResignation:
				if _, err := g.resign(ResignCommand{ProfileID: "white"}); err != nil {
					t.Fatal(err)
				}
			case domain.TerminationDrawAgreement:
				state, err := g.offerDraw(OfferDrawCommand{ProfileID: "white"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := g.acceptDraw(DrawOfferResponseCommand{ProfileID: "black", OfferID: state.PendingDrawOffer.OfferID}); err != nil {
					t.Fatal(err)
				}
			case domain.TerminationTimeout:
				now := g.clock.turnStartedAt.Add(time.Hour)
				g.now = func() time.Time { return now }
			}
			state := g.snapshot()
			for range 2 {
				if _, err := g.resume("white"); err != nil {
					t.Fatal(err)
				}
				g.snapshot()
				g.notifyExpiration(nil)
			}
			finished := 0
			for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
				if len(line) == 0 {
					continue
				}
				var event struct {
					Event       string
					GameID      identity.GameID `json:"game_id"`
					Outcome     domain.Outcome
					Termination domain.Termination
					Version     uint64
				}
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal(err)
				}
				if event.Event == "game.finished" {
					finished++
					if event.GameID != g.id || event.Outcome != state.Outcome || event.Termination != ending || event.Version != state.Version {
						t.Fatalf("completion event = %+v, state = %+v", event, state)
					}
				}
			}
			if finished != 1 {
				t.Fatalf("completion events = %d, want one despite repeated reads", finished)
			}
		})
	}
}
