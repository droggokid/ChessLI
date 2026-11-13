package gameplay

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	domain "ChessLI/internal/game"
)

func TestGameServiceIgnoresStaleTimerAfterMoveFinishOrClose(t *testing.T) {
	for _, action := range []string{"move", "resign", "close"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				service := newTestGameService(t)
				defer service.Close()
				g := newReadyGame()
				now := time.Now()
				g.now = func() time.Time { return now }
				g.clock = newGameClock(time.Minute, 0)
				g.clock.start(now)
				service.games[g.id] = g
				notified := false
				service.SetGameExpiredHandler(func(GameSnapshot) { notified = true })
				service.mu.Lock()
				service.scheduleExpiration(g)
				id := g.expirationID
				deadline, _ := g.clock.deadline(g.engine.Position().Turn())
				service.mu.Unlock()
				switch action {
				case "move":
					if _, err := service.MakeMove(context.Background(), MoveCommand{GameID: g.id, ProfileID: "white", Move: "e2e4"}); err != nil {
						t.Fatal(err)
					}
				case "resign":
					if _, err := service.Resign(context.Background(), ResignCommand{GameID: g.id, ProfileID: "white"}); err != nil {
						t.Fatal(err)
					}
				case "close":
					service.Close()
				}
				service.mu.Lock()
				now = deadline
				service.mu.Unlock()
				service.expireFromTimer(g, id)
				synctest.Wait()
				service.mu.Lock()
				outcome := g.outcome
				service.mu.Unlock()
				want := domain.OutcomeNone
				if action == "resign" {
					want = domain.OutcomeBlackWin
				}
				if outcome != want || notified {
					t.Fatal("stale timer expired the game")
				}
			})
		})
	}
}
