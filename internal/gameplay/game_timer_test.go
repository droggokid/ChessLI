package gameplay

import (
	"testing"
	"testing/synctest"
	"time"

	domain "ChessLI/internal/game"
)

func TestGameIgnoresReplacedOrStoppedTimer(t *testing.T) {
	for _, action := range []string{"replace", "stop", "move"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				g := newReadyGame()
				defer g.stopExpiration()
				now := time.Now()
				g.now = func() time.Time { return now }
				g.clock = newGameClock(time.Minute, 0)
				g.clock.start(now)
				g.scheduleExpiration(nil)
				id, version := g.expirationID, g.version
				deadline, _ := g.clock.deadline(g.engine.Position().Turn())
				switch action {
				case "replace":
					g.scheduleExpiration(nil)
				case "stop":
					g.stopExpiration()
				case "move":
					if _, err := g.move(MoveCommand{ProfileID: "white", Move: "e2e4"}); err != nil {
						t.Fatal(err)
					}
				}
				now = deadline
				notified := false
				g.expireFromTimer(id, version, deadline, func(GameSnapshot) { notified = true })
				if g.outcome != domain.OutcomeNone || notified {
					t.Fatal("stale timer expired the game")
				}
			})
		})
	}
}
