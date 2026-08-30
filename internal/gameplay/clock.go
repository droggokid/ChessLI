package gameplay

import (
	"time"

	"github.com/corentings/chess/v2"
)

type gameClock struct {
	whiteRemaining time.Duration
	blackRemaining time.Duration
	increment      time.Duration
	turnStartedAt  time.Time
	started        bool
	running        bool
}

func newGameClock(initial, increment time.Duration) gameClock {
	return gameClock{
		whiteRemaining: initial,
		blackRemaining: initial,
		increment:      increment,
	}
}

func (c *gameClock) start(now time.Time) {
	if c.started {
		return
	}

	c.started = true
	c.running = true
	c.turnStartedAt = now
}

func (c *gameClock) remaining(now time.Time, active chess.Color) (white, black time.Duration) {
	white = c.whiteRemaining
	black = c.blackRemaining

	if !c.running {
		return white, black
	}

	elapsed := now.Sub(c.turnStartedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	switch active {
	case chess.White:
		white = clampDuration(white - elapsed)
	case chess.Black:
		black = clampDuration(black - elapsed)
	default:
		return white, black
	}

	return white, black
}

func (c *gameClock) expire(now time.Time, active chess.Color) (loser chess.Color, expired bool) {
	if !c.running {
		return chess.NoColor, false
	}

	white, black := c.remaining(now, active)

	switch active {
	case chess.White:
		if white > 0 {
			return chess.NoColor, false
		}
	case chess.Black:
		if black > 0 {
			return chess.NoColor, false
		}
	default:
		return chess.NoColor, false
	}

	c.whiteRemaining = white
	c.blackRemaining = black
	c.running = false

	return active, true
}

func (c *gameClock) deadline(active chess.Color) (time.Time, bool) {
	if !c.running {
		return time.Time{}, false
	}

	var remaining time.Duration

	switch active {
	case chess.White:
		remaining = c.whiteRemaining
	case chess.Black:
		remaining = c.blackRemaining
	default:
		return time.Time{}, false
	}

	return c.turnStartedAt.Add(remaining), true
}

// completeMove commits the mover's elapsed time, adds the increment, and
// starts the opponent's clock.
func (c *gameClock) completeMove(now time.Time, mover chess.Color) {
	if !c.running {
		return
	}

	white, black := c.remaining(now, mover)
	c.whiteRemaining = white
	c.blackRemaining = black

	switch mover {
	case chess.White:
		c.whiteRemaining += c.increment
	case chess.Black:
		c.blackRemaining += c.increment
	default:
		return
	}

	c.turnStartedAt = now
}

// stop commits the current elapsed time and stops both clocks.
func (c *gameClock) stop(now time.Time, active chess.Color) {
	if !c.running {
		return
	}

	c.whiteRemaining, c.blackRemaining = c.remaining(now, active)
	c.running = false
}

func clampDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}

	return value
}

func (g *Game) startClockIfReadyLocked() {
	if g.WhiteProfileID != "" && g.BlackProfileID != "" {
		g.clock.start(g.now())
	}
}

// scheduleExpiration replaces the current timer with one for the active turn.
func (g *Game) scheduleExpiration(onExpired func(GameSnapshot)) {
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

func (g *Game) expireFromTimer(expirationID uint64, expectedVersion uint64, deadline time.Time, onExpired func(GameSnapshot)) {
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

func (g *Game) stopExpiration() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.expirationTimer != nil {
		g.expirationTimer.Stop()
		g.expirationTimer = nil
	}
	g.expirationID++
}
