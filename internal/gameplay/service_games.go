package gameplay

import (
	"context"

	"ChessLI/internal/identity"
)

// MakeMove applies a move to the identified game on behalf of a profile.
func (s *GameService) MakeMove(ctx context.Context, command MoveCommand) (GameSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}

	state, err := game.Move(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)
	return state, nil
}

// GameState returns the current authoritative snapshot of a game.
func (s *GameService) GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(gameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	return game.Snapshot(), nil
}
