package gameplay

import (
	"fmt"
	"sync"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type Game struct {
	mu              sync.Mutex
	ID              identity.GameID
	version         uint64
	WhiteProfileID  identity.ProfileID
	BlackProfileID  identity.ProfileID
	engine          *chess.Game
	lastMoveSAN     string
	clock           gameClock
	expirationTimer *time.Timer
	expirationID    uint64
	now             func() time.Time
	outcome         chess.Outcome
	termination     TerminationReason
}

// NewGame returns a game with the standard starting position and the supplied
// players and time control.
func NewGame(id identity.GameID, white identity.ProfileID, black identity.ProfileID, initialTime time.Duration, increment time.Duration) *Game {
	game := &Game{
		ID:             id,
		version:        0,
		WhiteProfileID: white,
		BlackProfileID: black,
		engine:         chess.NewGame(),
		clock:          newGameClock(initialTime, increment),
		now:            time.Now,
		outcome:        chess.NoOutcome,
		termination:    TerminationNone,
	}

	if white != "" && black != "" {
		game.clock.start(game.now())
	}

	return game
}

// Move decodes, validates, and applies a move for the profile whose turn it is.
func (g *Game) Move(command MoveCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()

	if g.expireLocked(now) {
		return g.snapshotLocked(now), nil
	}

	if command.ExpectedVersion != g.version {
		return GameSnapshot{}, ErrStaleGameVersion
	}

	if err := g.validateMove(command.ProfileID); err != nil {
		return GameSnapshot{}, err
	}

	position := g.engine.Position()

	notation, err := engineNotation(command.Notation)
	if err != nil {
		return GameSnapshot{}, err
	}

	move, err := notation.Decode(position, command.Move)
	if err != nil {
		return GameSnapshot{}, fmt.Errorf("%w: %v", ErrIllegalMove, err)
	}

	san := chess.AlgebraicNotation{}.Encode(position, move)
	mover := position.Turn()

	if err = g.engine.Move(move, nil); err != nil {
		return GameSnapshot{}, ErrIllegalMove
	}

	g.clock.completeMove(now, mover)
	g.syncOutcomeFromEngineLocked()

	if g.outcome != chess.NoOutcome {
		g.clock.stop(now, g.engine.Position().Turn())
	}

	g.version++
	g.lastMoveSAN = san

	return g.snapshotLocked(now), nil
}

func (g *Game) validateMove(source identity.ProfileID) error {
	if err := g.validateReady(); err != nil {
		return err
	}

	if g.outcome != chess.NoOutcome {
		return ErrGameFinished
	}

	turn := g.engine.CurrentPosition().Turn()

	switch turn {
	case chess.White:
		if source != g.WhiteProfileID {
			return ErrNotYourTurn
		}
	case chess.Black:
		if source != g.BlackProfileID {
			return ErrNotYourTurn
		}
	default:
		return ErrGameFinished
	}

	return nil
}

func (g *Game) validateReady() error {
	if g.WhiteProfileID == "" || g.BlackProfileID == "" {
		return ErrGameNotReady
	}

	return nil
}

// JoinPrivate assigns a profile to the unoccupied color in a private game.
func (g *Game) JoinPrivate(command JoinPrivateCommand) (chess.Color, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch {
	case g.WhiteProfileID == command.ProfileID || g.BlackProfileID == command.ProfileID:
		return chess.NoColor, ErrAlreadyParticipant

	case g.WhiteProfileID == "":
		g.WhiteProfileID = command.ProfileID
		g.startClockIfReadyLocked()
		return chess.White, nil

	case g.BlackProfileID == "":
		g.BlackProfileID = command.ProfileID
		g.startClockIfReadyLocked()
		return chess.Black, nil

	default:
		return chess.NoColor, ErrGameFull
	}
}

// Snapshot returns a consistent copy of the game's current authoritative state.
func (g *Game) Snapshot() GameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	return g.snapshotLocked(now)
}

func (g *Game) snapshotLocked(now time.Time) GameSnapshot {
	whiteRemaining, blackRemaining := g.clock.remaining(now, g.engine.Position().Turn())

	return GameSnapshot{
		GameID:         g.ID,
		FEN:            g.engine.FEN(),
		Version:        g.version,
		WhiteProfileID: g.WhiteProfileID,
		BlackProfileID: g.BlackProfileID,
		WhiteRemaining: whiteRemaining,
		BlackRemaining: blackRemaining,
		LastMoveSAN:    g.lastMoveSAN,
		Outcome:        g.outcome,
		Termination:    g.termination,
	}
}
