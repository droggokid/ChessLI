package gameplay

import (
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// createMatchLocked creates and stores a game for two matched players.
// The caller must hold s.mu.
func (s *GameService) createMatchLocked(waiting, current *waitingPlayer) (MatchResult, MatchResult, *Game, error) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID, err := assignMatchmakingColors(
		waiting.command.ProfileID,
		current.command.ProfileID,
		waitingColor,
	)
	if err != nil {
		return MatchResult{}, MatchResult{}, nil, err
	}

	gameID := identity.NewGameID()
	timeControl := current.command.timeControl
	game := NewGame(gameID, whiteProfileID, blackProfileID, timeControl.initial, timeControl.increment)
	s.games[gameID] = game

	return MatchResult{GameID: gameID, Color: waitingColor},
		MatchResult{GameID: gameID, Color: currentColor},
		game,
		nil
}

func assignMatchmakingColors(
	waitingProfileID identity.ProfileID,
	currentProfileID identity.ProfileID,
	waitingColor chess.Color,
) (identity.ProfileID, identity.ProfileID, error) {
	switch waitingColor {
	case chess.White:
		return waitingProfileID, currentProfileID, nil
	case chess.Black:
		return currentProfileID, waitingProfileID, nil
	default:
		return "", "", ErrInvalidColorPreference
	}
}
