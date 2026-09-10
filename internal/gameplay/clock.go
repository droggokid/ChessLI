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
	if c.running {
		return
	}

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
