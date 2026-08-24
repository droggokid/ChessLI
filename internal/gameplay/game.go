package gameplay

import (
	"fmt"
	"sync"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type Game struct {
	mu             sync.Mutex
	ID             identity.GameID
	version        uint64
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	engine         *chess.Game
	lastMoveSAN    string
	whiteRemaining time.Duration
	blackRemaining time.Duration
	increment      time.Duration
}

// NewGame returns a game with the standard starting position and the supplied
// players and time control.
func NewGame(id identity.GameID, white identity.ProfileID, black identity.ProfileID, initialTime time.Duration, increment time.Duration) *Game {
	return &Game{
		ID:             id,
		version:        0,
		WhiteProfileID: white,
		BlackProfileID: black,
		engine:         chess.NewGame(),
		whiteRemaining: initialTime,
		blackRemaining: initialTime,
		increment:      increment,
	}
}

// Move decodes, validates, and applies a move for the profile whose turn it is.
func (g *Game) Move(command MoveCommand) (MoveResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if command.ExpectedVersion != g.version {
		return MoveResult{}, ErrStaleGameVersion
	}

	if err := g.validateMove(command.ProfileID); err != nil {
		return MoveResult{}, err
	}

	position := g.engine.Position()

	notation, err := engineNotation(command.Notation)
	if err != nil {
		return MoveResult{}, err
	}

	move, err := notation.Decode(position, command.Move)
	if err != nil {
		return MoveResult{}, fmt.Errorf("%w: %v", ErrIllegalMove, err)
	}

	san := chess.AlgebraicNotation{}.Encode(position, move)

	if err = g.engine.Move(move, nil); err != nil {
		return MoveResult{}, ErrIllegalMove
	}

	g.version++
	g.lastMoveSAN = san

	return MoveResult{
		GameID:  g.ID,
		FEN:     g.engine.FEN(),
		SAN:     san,
		Outcome: g.engine.Outcome(),
		Method:  g.engine.Method(),
		Version: g.version,
	}, nil
}

func (g *Game) validateMove(source identity.ProfileID) error {
	if err := g.validateReady(); err != nil {
		return err
	}

	if g.engine.Outcome() != chess.NoOutcome {
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

func engineNotation(notation MoveNotation) (chess.Notation, error) {
	switch notation {
	case MoveNotationUCI:
		return chess.UCINotation{}, nil
	case MoveNotationSAN:
		return chess.AlgebraicNotation{}, nil
	case MoveNotationLAN:
		return chess.LongAlgebraicNotation{}, nil
	default:
		return nil, ErrUnsupportedNotation
	}
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
		return chess.White, nil

	case g.BlackProfileID == "":
		g.BlackProfileID = command.ProfileID
		return chess.Black, nil

	default:
		return chess.NoColor, ErrGameFull
	}
}

// Snapshot returns a consistent copy of the game's current authoritative state.
func (g *Game) Snapshot() GameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.snapshotLocked()
}

func (g *Game) snapshotLocked() GameSnapshot {
	return GameSnapshot{
		GameID:         g.ID,
		FEN:            g.engine.FEN(),
		Version:        g.version,
		WhiteProfileID: g.WhiteProfileID,
		BlackProfileID: g.BlackProfileID,
		WhiteRemaining: g.whiteRemaining,
		BlackRemaining: g.blackRemaining,
		LastMoveSAN:    g.lastMoveSAN,
		Outcome:        g.engine.Outcome(),
		Method:         g.engine.Method(),
	}
}
