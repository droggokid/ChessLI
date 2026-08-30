package gameplay

import (
	"testing"
	"time"

	"github.com/corentings/chess/v2"
)

func TestGameClockRemainingTime(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	clock := newGameClock(10*time.Minute, 2*time.Second)

	white, black := clock.remaining(base.Add(time.Hour), chess.White)
	if white != 10*time.Minute || black != 10*time.Minute {
		t.Fatalf("remaining before start = (%v, %v), want 10m each", white, black)
	}

	clock.start(base)
	white, black = clock.remaining(base.Add(5*time.Second), chess.White)
	if white != 9*time.Minute+55*time.Second || black != 10*time.Minute {
		t.Fatalf("remaining during White turn = (%v, %v), want (9m55s, 10m)", white, black)
	}
}

func TestGameClockCompleteMoveAddsIncrementAndSwitchesClock(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	clock := newGameClock(time.Minute, 2*time.Second)
	clock.start(base)
	clock.completeMove(base.Add(5*time.Second), chess.White)

	white, black := clock.remaining(base.Add(8*time.Second), chess.Black)
	if white != 57*time.Second || black != 57*time.Second {
		t.Fatalf("remaining after move = (%v, %v), want 57s each", white, black)
	}
}

func TestGameClockExpireStopsAtZero(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	clock := newGameClock(time.Second, 0)
	clock.start(base)

	loser, expired := clock.expire(base.Add(time.Second), chess.White)
	if !expired || loser != chess.White {
		t.Fatalf("expire() = (%v, %v), want (White, true)", loser, expired)
	}

	white, black := clock.remaining(base.Add(time.Hour), chess.Black)
	if white != 0 || black != time.Second {
		t.Fatalf("remaining after expiration = (%v, %v), want (0, 1s)", white, black)
	}

	if loser, expired = clock.expire(base.Add(time.Hour), chess.Black); expired || loser != chess.NoColor {
		t.Fatalf("second expire() = (%v, %v), want (NoColor, false)", loser, expired)
	}
}

func TestGameClockStopCommitsElapsedTime(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	clock := newGameClock(time.Minute, 0)
	clock.start(base)
	clock.stop(base.Add(4*time.Second), chess.White)

	white, black := clock.remaining(base.Add(time.Hour), chess.White)
	if white != 56*time.Second || black != time.Minute {
		t.Fatalf("remaining after stop = (%v, %v), want (56s, 1m)", white, black)
	}
}

func TestGameClockDeadlineUsesActivePlayersRemainingTime(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	clock := newGameClock(time.Minute, 0)

	if _, ok := clock.deadline(chess.White); ok {
		t.Fatal("deadline() reported a deadline before the clock started")
	}

	clock.start(base)
	clock.completeMove(base.Add(5*time.Second), chess.White)

	deadline, ok := clock.deadline(chess.Black)
	if !ok {
		t.Fatal("deadline() did not report a running clock deadline")
	}

	want := base.Add(65 * time.Second)
	if !deadline.Equal(want) {
		t.Fatalf("deadline() = %v, want %v", deadline, want)
	}
}
