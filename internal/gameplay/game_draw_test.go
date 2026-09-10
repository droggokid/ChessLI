package gameplay

import (
	"errors"
	"testing"
	"time"

	"github.com/corentings/chess/v2"
)

func TestGameDrawOfferLifecycle(t *testing.T) {
	base := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	now := base
	game := newReadyGame()
	game.now = func() time.Time { return now }

	if _, err := game.OfferDraw(NewOfferDrawCommand(game.ID, "spectator")); !errors.Is(err, ErrNotParticipant) {
		t.Fatalf("OfferDraw() spectator error = %v, want %v", err, ErrNotParticipant)
	}

	offered, err := game.OfferDraw(NewOfferDrawCommand(game.ID, "white"))
	if err != nil {
		t.Fatalf("OfferDraw() error = %v", err)
	}
	if offered.Version != 0 || offered.PendingDrawOffer == nil {
		t.Fatalf("OfferDraw() state = %+v, want pending offer at unchanged version", offered)
	}
	offerID := offered.PendingDrawOffer.OfferID
	if _, err = game.OfferDraw(NewOfferDrawCommand(game.ID, "black")); !errors.Is(err, ErrDrawOfferPending) {
		t.Fatalf("OfferDraw() pending-offer error = %v, want %v", err, ErrDrawOfferPending)
	}

	if _, err = game.AcceptDraw(NewDrawOfferResponseCommand(game.ID, "white", offerID)); !errors.Is(err, ErrCannotRespondToOwnDrawOffer) {
		t.Fatalf("AcceptDraw() own-offer error = %v, want %v", err, ErrCannotRespondToOwnDrawOffer)
	}
	if _, err = game.AcceptDraw(NewDrawOfferResponseCommand(game.ID, "black", "stale")); !errors.Is(err, ErrStaleDrawOffer) {
		t.Fatalf("AcceptDraw() stale-offer error = %v, want %v", err, ErrStaleDrawOffer)
	}

	declined, err := game.DeclineDraw(NewDrawOfferResponseCommand(game.ID, "black", offerID))
	if err != nil {
		t.Fatalf("DeclineDraw() error = %v", err)
	}
	if declined.Version != 0 || declined.PendingDrawOffer != nil {
		t.Fatalf("DeclineDraw() state = %+v, want cleared offer at unchanged version", declined)
	}
	if _, err = game.DeclineDraw(NewDrawOfferResponseCommand(game.ID, "black", offerID)); !errors.Is(err, ErrDrawOfferNotFound) {
		t.Fatalf("DeclineDraw() missing-offer error = %v, want %v", err, ErrDrawOfferNotFound)
	}
	if _, err = game.OfferDraw(NewOfferDrawCommand(game.ID, "white")); !errors.Is(err, ErrDrawOfferCooldown) {
		t.Fatalf("OfferDraw() cooldown error = %v, want %v", err, ErrDrawOfferCooldown)
	}

	now = now.Add(drawOfferCooldown)
	offered, err = game.OfferDraw(NewOfferDrawCommand(game.ID, "white"))
	if err != nil {
		t.Fatalf("OfferDraw() after cooldown error = %v", err)
	}
	offerID = offered.PendingDrawOffer.OfferID
	offered.PendingDrawOffer.OfferedBy = "tampered"
	if got := game.Snapshot().PendingDrawOffer.OfferedBy; got != "white" {
		t.Fatalf("Snapshot() pending offer was aliased: OfferedBy = %q", got)
	}

	accepted, err := game.AcceptDraw(NewDrawOfferResponseCommand(game.ID, "black", offerID))
	if err != nil {
		t.Fatalf("AcceptDraw() error = %v", err)
	}
	if accepted.Version != 1 || accepted.Outcome != chess.Draw || accepted.Termination != TerminationDrawAgreement || accepted.PendingDrawOffer != nil {
		t.Fatalf("AcceptDraw() state = %+v, want finished draw at version 1", accepted)
	}
}

func TestGameClearsPendingDrawOffer(t *testing.T) {
	tests := []struct {
		name string
		act  func(*Game) (GameSnapshot, error)
	}{
		{
			name: "move",
			act: func(game *Game) (GameSnapshot, error) {
				return game.Move(NewMoveCommand(game.ID, "white", "e2e4", MoveNotationUCI, 0))
			},
		},
		{
			name: "resignation",
			act: func(game *Game) (GameSnapshot, error) {
				return game.Resign(NewResignCommand(game.ID, "black"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := newReadyGame()
			if _, err := game.OfferDraw(NewOfferDrawCommand(game.ID, "white")); err != nil {
				t.Fatalf("OfferDraw() error = %v", err)
			}

			state, err := tt.act(game)
			if err != nil {
				t.Fatalf("action error = %v", err)
			}
			if state.PendingDrawOffer != nil {
				t.Fatalf("action left pending offer %+v", state.PendingDrawOffer)
			}
		})
	}

	t.Run("timeout", func(t *testing.T) {
		base := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
		now := base
		game := NewGame("game", "white", "black", time.Second, 0)
		game.now = func() time.Time { return now }
		game.clock = newGameClock(time.Second, 0)
		game.clock.start(now)
		if _, err := game.OfferDraw(NewOfferDrawCommand(game.ID, "white")); err != nil {
			t.Fatalf("OfferDraw() error = %v", err)
		}

		now = now.Add(time.Second)
		if state := game.Snapshot(); state.PendingDrawOffer != nil {
			t.Fatalf("timeout left pending offer %+v", state.PendingDrawOffer)
		}
	})
}
