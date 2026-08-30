package gameplay

import (
	"context"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type waitingPlayer struct {
	command EnterMatchmakingCommand
	result  chan MatchResult
	done    <-chan struct{}
}

// timeControlKey identifies one matchmaking pool by its clock settings.
type timeControlKey struct {
	initial   time.Duration
	increment time.Duration
}

// newWaitingPlayer creates a queue entry that is canceled with ctx.
func newWaitingPlayer(ctx context.Context, command EnterMatchmakingCommand) *waitingPlayer {
	return &waitingPlayer{command: command, result: make(chan MatchResult, 1), done: ctx.Done()}
}

// findOpponentLocked matches player within its time-control pool or queues it.
// The caller must hold s.mu.
func (s *GameService) findOpponentLocked(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	s.removeStaleWaitingPlayerLocked()

	for _, waiting := range s.waitingPlayers {
		if waiting.command.ProfileID == player.command.ProfileID {
			return nil, false, ErrAlreadyQueued
		}
	}

	key := player.command.timeControl
	waiting, exists := s.waitingPlayers[key]
	if !exists {
		s.waitingPlayers[key] = player
		return nil, true, nil
	}

	delete(s.waitingPlayers, key)

	return waiting, false, nil
}

// removeStaleWaitingPlayerLocked removes canceled players from every pool.
// The caller must hold s.mu.
func (s *GameService) removeStaleWaitingPlayerLocked() {
	for key, waiting := range s.waitingPlayers {
		if !playerDone(waiting.done) {
			continue
		}

		delete(s.waitingPlayers, key)
		close(waiting.result)
	}
}

// createMatchLocked creates and stores a game for two matched players.
// The caller must hold s.mu.
func (s *GameService) createMatchLocked(waiting *waitingPlayer, current *waitingPlayer) (MatchResult, MatchResult, *Game) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID := assignMatchmakingColors(waiting.command.ProfileID, current.command.ProfileID, waitingColor)

	gameID := identity.NewGameID()
	timeControl := current.command.timeControl

	game := NewGame(gameID, whiteProfileID, blackProfileID, timeControl.initial, timeControl.increment)
	s.games[gameID] = game

	return MatchResult{GameID: gameID, Color: waitingColor}, MatchResult{GameID: gameID, Color: currentColor}, game
}

// deliverMatchResult sends one match result and completes its ticket.
func deliverMatchResult(resultChannel chan MatchResult, result MatchResult) {
	resultChannel <- result
	close(resultChannel)
}

// assignMatchmakingColors returns profile IDs ordered as White, then Black.
func assignMatchmakingColors(waitingProfileID identity.ProfileID, currentProfileID identity.ProfileID, waitingColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if waitingColor == chess.White {
		return waitingProfileID, currentProfileID
	}

	return currentProfileID, waitingProfileID
}

// removeWaitingPlayerOnCancel unregisters a queued player when its context ends.
func (s *GameService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	if player.done == nil {
		return
	}

	<-player.done

	s.mu.Lock()
	defer s.mu.Unlock()

	key := player.command.timeControl
	// The player may already have been matched. Only remove it
	// when it is still the exact waiting entry.
	if s.waitingPlayers[key] != player {
		return
	}

	delete(s.waitingPlayers, key)
	close(player.result)
}

// playerDone reports whether a player's cancellation signal has fired.
func playerDone(done <-chan struct{}) bool {
	if done == nil {
		return false
	}

	select {
	case <-done:
		return true
	default:
		return false
	}
}
