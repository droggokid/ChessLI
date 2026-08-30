package gameplay

import (
	"context"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// CreatePrivateGame creates a private game using the requested time control and color preference.
func (s *GameService) CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}
	if err := validateTimeControl(command.Initial, command.Increment); err != nil {
		return CreateResult{}, err
	}

	creatorColor, err := s.handleColorPreference(command.ColorPreference)
	if err != nil {
		return CreateResult{}, err
	}

	whiteProfileID, blackProfileID := assignPrivateColors(command.ProfileID, creatorColor)
	gameID := identity.NewGameID()
	game := NewGame(gameID, whiteProfileID, blackProfileID, command.Initial, command.Increment)

	s.mu.Lock()
	s.games[gameID] = game
	s.mu.Unlock()

	return CreateResult{GameID: gameID, Color: creatorColor}, nil
}

// JoinPrivateGame seats a profile in the open position of a private game.
func (s *GameService) JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error) {
	if err := ctx.Err(); err != nil {
		return JoinResult{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return JoinResult{}, err
	}

	color, err := game.JoinPrivate(command)
	if err != nil {
		return JoinResult{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)
	return JoinResult{GameID: game.ID, Color: color}, nil
}

func validateTimeControl(initial, increment time.Duration) error {
	if initial <= 0 || increment < 0 {
		return ErrInvalidTimeControl
	}
	return nil
}

func (s *GameService) handleColorPreference(preference ColorPreference) (chess.Color, error) {
	switch preference {
	case ColorWhite:
		return chess.White, nil
	case ColorBlack:
		return chess.Black, nil
	case ColorRandom:
		return s.pickColor(), nil
	default:
		return chess.NoColor, ErrInvalidColorPreference
	}
}

func assignPrivateColors(profileID identity.ProfileID, creatorColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if creatorColor == chess.White {
		return profileID, ""
	}
	return "", profileID
}
