package gameplay

import (
	"fmt"
	"sync"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// game is safe for concurrent use through its methods.
type game struct {
	// mu guards all game state. Helpers ending in Locked require callers to hold it and do not lock it.
	mu              sync.Mutex
	id              identity.GameID
	version         uint64
	whiteProfileID  identity.ProfileID
	blackProfileID  identity.ProfileID
	engine          *chess.Game
	lastMoveSAN     string
	clock           gameClock
	expirationTimer *time.Timer
	expirationID    uint64
	now             func() time.Time
	outcome         chess.Outcome
	termination     TerminationReason
	drawOffers      drawOfferState
}

// newGame returns a game with the standard starting position and the supplied
// players and time control.
func newGame(id identity.GameID, white identity.ProfileID, black identity.ProfileID, initialTime time.Duration, increment time.Duration) *game {
	g := &game{
		id:             id,
		version:        0,
		whiteProfileID: white,
		blackProfileID: black,
		engine:         chess.NewGame(),
		clock:          newGameClock(initialTime, increment),
		now:            time.Now,
		outcome:        chess.NoOutcome,
		termination:    TerminationNone,
		drawOffers: drawOfferState{
			lastOfferedAt: make(map[identity.ProfileID]time.Time, 2),
		},
	}

	if white != "" && black != "" {
		g.clock.start(g.now())
	}

	return g
}

// move decodes, validates, and applies a move for the profile whose turn it is.
func (g *game) move(command MoveCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()

	if g.expireLocked(now) {
		return g.snapshotLocked(now), nil
	}

	if command.ExpectedVersion != g.version {
		return GameSnapshot{}, ErrStaleGameVersion
	}

	if err := g.validateMoveLocked(command.ProfileID); err != nil {
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
	g.clearPendingDrawOfferLocked()

	return g.snapshotLocked(now), nil
}

func (g *game) validateMoveLocked(source identity.ProfileID) error {
	if err := g.validateReadyLocked(); err != nil {
		return err
	}

	if g.outcome != chess.NoOutcome {
		return ErrGameFinished
	}
	if source != g.whiteProfileID && source != g.blackProfileID {
		return ErrNotParticipant
	}

	turn := g.engine.CurrentPosition().Turn()

	switch turn {
	case chess.White:
		if source != g.whiteProfileID {
			return ErrNotYourTurn
		}
	case chess.Black:
		if source != g.blackProfileID {
			return ErrNotYourTurn
		}
	default:
		return ErrGameFinished
	}

	return nil
}

func (g *game) validateReadyLocked() error {
	if g.whiteProfileID == "" || g.blackProfileID == "" {
		return ErrGameNotReady
	}

	return nil
}

// joinPrivate assigns a profile to the unoccupied color in a private game.
func (g *game) joinPrivate(command JoinPrivateCommand) (chess.Color, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch {
	case g.whiteProfileID == command.ProfileID || g.blackProfileID == command.ProfileID:
		return chess.NoColor, ErrAlreadyParticipant

	case g.whiteProfileID == "":
		g.whiteProfileID = command.ProfileID
		g.startClockIfReadyLocked()
		return chess.White, nil

	case g.blackProfileID == "":
		g.blackProfileID = command.ProfileID
		g.startClockIfReadyLocked()
		return chess.Black, nil

	default:
		return chess.NoColor, ErrGameFull
	}
}

func (g *game) startClockIfReadyLocked() {
	if g.whiteProfileID != "" && g.blackProfileID != "" {
		g.clock.start(g.now())
	}
}

// resign resigns a game participant.
func (g *game) resign(command ResignCommand) (GameSnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	if err := g.validateReadyLocked(); err != nil {
		return GameSnapshot{}, err
	}
	if g.expireLocked(now) || g.outcome != chess.NoOutcome {
		return GameSnapshot{}, ErrGameFinished
	}

	color, err := colorFromProfileID(command.ProfileID, g.whiteProfileID, g.blackProfileID)
	if err != nil {
		return GameSnapshot{}, err
	}

	g.engine.Resign(color)
	g.syncOutcomeFromEngineLocked()
	g.version++
	g.clock.stop(now, g.engine.Position().Turn())
	g.clearPendingDrawOfferLocked()

	return g.snapshotLocked(now), nil
}
