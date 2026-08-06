package gameplay

import (
	"fmt"
	"sync"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type Game struct {
	mu sync.Mutex

	ID identity.GameID

	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID

	engine *chess.Game

	whiteRemaining time.Duration
	blackRemaining time.Duration
	increment      time.Duration
}

// NewGame returns a game with the standard starting position and the supplied
// players and time control.
func NewGame(id identity.GameID, white identity.ProfileID, black identity.ProfileID, initialTime time.Duration, increment time.Duration) *Game {
	return &Game{
		ID:             id,
		WhiteProfileID: white,
		BlackProfileID: black,
		engine:         chess.NewGame(),
		whiteRemaining: initialTime,
		blackRemaining: initialTime,
		increment:      increment,
	}
}

// Move validates and applies a UCI move for the profile whose turn it is.
func (g *Game) Move(command MoveCommand) (MoveResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.validateMove(command.ProfileID); err != nil {
		return MoveResult{}, err
	}

	if err := g.engine.PushNotationMove(
		command.Move,
		chess.UCINotation{},
		nil,
	); err != nil {
		return MoveResult{}, fmt.Errorf("%w: %v", ErrIllegalMove, err)
	}

	return MoveResult{
		GameID:  g.ID,
		FEN:     g.engine.FEN(),
		Outcome: g.engine.Outcome(),
		Method:  g.engine.Method(),
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
