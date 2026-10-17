package gameplay

import (
	"time"

	"github.com/corentings/chess/v2"
)

// gameClock receives White or Black from the engine and monotonic game times.
// gameState starts it once when both seats are filled, and moves/stops it while running.
type gameClock struct {
	whiteRemaining time.Duration
	blackRemaining time.Duration
	increment      time.Duration
	turnStartedAt  time.Time
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
	c.running = true
	c.turnStartedAt = now
}

// remaining calculates the current clock values without modifying stored time.
// Only the active player's time decreases while the clock is running.
func (c *gameClock) remaining(now time.Time, active chess.Color) (white, black time.Duration) {
	white = c.whiteRemaining
	black = c.blackRemaining

	if !c.running {
		return white, black
	}

	// Time elapsed since the active player's turn began.
	// Clamp to zero so the displayed clock never becomes negative.
	elapsed := now.Sub(c.turnStartedAt)
	if active == chess.White {
		white = max(white-elapsed, 0)
	} else {
		black = max(black-elapsed, 0)
	}

	return white, black
}

// expire checks whether the active player has run out of time.
// If so, it commits the remaining times and stops the clock.
func (c *gameClock) expire(now time.Time, active chess.Color) (loser chess.Color, expired bool) {
	if !c.running {
		return chess.NoColor, false
	}

	white, black := c.remaining(now, active)

	remaining := black
	if active == chess.White {
		remaining = white
	}
	if remaining > 0 {
		return chess.NoColor, false
	}

	c.whiteRemaining = white
	c.blackRemaining = black
	c.running = false

	return active, true
}

// deadline returns the exact time at which the active player will flag.
// Useful for scheduling a timeout without continuously polling the clock.
func (c *gameClock) deadline(active chess.Color) (time.Time, bool) {
	if !c.running {
		return time.Time{}, false
	}

	remaining := c.blackRemaining
	if active == chess.White {
		remaining = c.whiteRemaining
	}

	return c.turnStartedAt.Add(remaining), true
}

// completeMove deducts the mover's elapsed time, awards the increment,
// and starts measuring time for the opponent's turn.
func (c *gameClock) completeMove(now time.Time, mover chess.Color) {
	// Persist the elapsed time before switching turns.
	white, black := c.remaining(now, mover)
	c.whiteRemaining = white
	c.blackRemaining = black

	// Fischer increment: added after the player completes their move.
	if mover == chess.White {
		c.whiteRemaining += c.increment
	} else {
		c.blackRemaining += c.increment
	}

	// The opponent's clock starts counting down from this moment.
	c.turnStartedAt = now
}

// stop commits the active player's elapsed time and freezes both clocks.
func (c *gameClock) stop(now time.Time, active chess.Color) {
	c.whiteRemaining, c.blackRemaining = c.remaining(now, active)
	c.running = false
}
