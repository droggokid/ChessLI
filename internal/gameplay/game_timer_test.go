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
				g.state.clock = newGameClock(time.Minute, 0)
				g.state.clock.start(now)
				g.scheduleExpiration(func(GameSnapshot) {})
				id, version := g.expirationID, g.state.version
				deadline, _ := g.state.clock.deadline(g.state.engine.Position().Turn())
				switch action {
				case "replace":
					g.scheduleExpiration(func(GameSnapshot) {})
				case "stop":
					g.stopExpiration()
				case "move":
					if _, err := g.move(MoveCommand{ProfileID: "white", Move: "e2e4"}); err != nil {
						t.Fatal(err)
					}
				}
				now = deadline
				notified := false
				g.expireFromTimer(id, version, func(GameSnapshot) { notified = true })
				if g.state.outcome != domain.OutcomeNone || notified {
					t.Fatal("stale timer expired the game")
				}
			})
		})
	}
}
