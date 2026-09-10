package gameplay

import (
	"time"

	"github.com/corentings/chess/v2"
)

// scheduleExpiration replaces the real timer for the active turn.
func (g *game) scheduleExpiration(onExpired func(GameSnapshot)) {
	g.mu.Lock()

	if g.expirationTimer != nil {
		g.expirationTimer.Stop()
		g.expirationTimer = nil
	}
	g.expirationID++

	deadline, ok := g.clock.deadline(g.engine.Position().Turn())
	if !ok || g.outcome != chess.NoOutcome {
		g.mu.Unlock()
		return
	}

	expectedVersion := g.version
	expirationID := g.expirationID
	delay := deadline.Sub(g.now())
	if delay < 0 {
		delay = 0
	}

	g.expirationTimer = time.AfterFunc(delay, func() {
		g.expireFromTimer(expirationID, expectedVersion, deadline, onExpired)
	})

	g.mu.Unlock()
}

func (g *game) expireFromTimer(expirationID, expectedVersion uint64, deadline time.Time, onExpired func(GameSnapshot)) {
	g.mu.Lock()

	if g.expirationID != expirationID || g.version != expectedVersion || g.outcome != chess.NoOutcome {
		g.mu.Unlock()
		return
	}

	currentDeadline, ok := g.clock.deadline(g.engine.Position().Turn())
	if !ok || !currentDeadline.Equal(deadline) {
		g.mu.Unlock()
		return
	}

	now := g.now()
	if now.Before(deadline) {
		g.mu.Unlock()
		g.scheduleExpiration(onExpired)
		return
	}

	g.expirationTimer = nil
	expired := g.expireLocked(now)
	state := g.snapshotLocked(now)
	g.mu.Unlock()

	if expired && onExpired != nil {
		onExpired(state)
	}
}

func (g *game) stopExpiration() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.expirationTimer != nil {
		g.expirationTimer.Stop()
		g.expirationTimer = nil
	}
	g.expirationID++
}
