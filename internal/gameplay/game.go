package gameplay

import (
	"sync"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// game owns synchronization and timers; all access to state is protected by mu.
type game struct {
	mu                 sync.Mutex
	state              gameState
	expirationTimer    *time.Timer
	expirationID       uint64
	expirationNotified bool
	now                func() time.Time
}

func newGame(id identity.GameID, white, black identity.ProfileID, initial, increment time.Duration) *game {
	g := &game{now: time.Now}
	g.state = newGameState(id, white, black, initial, increment, g.now())
	return g
}

func (g *game) move(command MoveCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.move(command, g.now())
}

func (g *game) joinPrivate(command JoinPrivateCommand) (chess.Color, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.joinPrivate(command, g.now())
}

func (g *game) resign(command ResignCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.resign(command, g.now())
}

func (g *game) resume(profileID identity.ProfileID) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.resume(profileID, g.now())
}

func (g *game) offerDraw(command OfferDrawCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.offerDraw(command, g.now())
}

func (g *game) acceptDraw(command DrawOfferResponseCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.acceptDraw(command, g.now())
}

func (g *game) declineDraw(command DrawOfferResponseCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state.declineDraw(command, g.now())
}

// active checks elapsed time without settling or notifying a timeout.
func (g *game) active() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state.outcome != domain.OutcomeNone {
		return false
	}

	deadline, running := g.state.clock.deadline(g.state.engine.Position().Turn())
	return !running || g.now().Before(deadline)
}

// snapshot also settles an expired clock, so it needs exclusive access.
func (g *game) snapshot() GameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.state.expire(now)
	return g.state.snapshot(now)
}

// scheduleExpiration replaces the real timer for the active turn.
func (g *game) scheduleExpiration(onExpired func(GameSnapshot)) {
	g.mu.Lock()

	// Stop cannot retract an already running callback; the generation check does.
	if g.expirationTimer != nil {
		g.expirationTimer.Stop()
		g.expirationTimer = nil
	}
	g.expirationID++

	deadline, ok := g.state.clock.deadline(g.state.engine.Position().Turn())
	if !ok {
		g.mu.Unlock()
		return
	}

	expectedVersion := g.state.version
	expirationID := g.expirationID
	g.expirationTimer = time.AfterFunc(deadline.Sub(g.now()), func() {
		g.expireFromTimer(expirationID, expectedVersion, onExpired)
	})

	g.mu.Unlock()
}

func (g *game) expireFromTimer(expirationID, expectedVersion uint64, onExpired func(GameSnapshot)) {
	g.mu.Lock()

	// Moves and terminal results change version; replacing or stopping a timer changes its ID.
	if g.expirationID != expirationID || g.state.version != expectedVersion {
		g.mu.Unlock()
		return
	}

	g.expirationTimer = nil
	g.state.expire(g.now())
	g.mu.Unlock()
	g.notifyExpiration(onExpired)
}

func (g *game) notifyExpiration(onExpired func(GameSnapshot)) {
	g.mu.Lock()
	// Reads and commands can discover a timeout before the timer callback does.
	if g.state.termination != domain.TerminationTimeout || g.expirationNotified {
		g.mu.Unlock()
		return
	}
	g.expirationNotified = true
	state := g.state.snapshot(g.now())
	g.mu.Unlock()

	onExpired(state)
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
