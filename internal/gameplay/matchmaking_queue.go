package gameplay

import (
	"context"
	"time"
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

func newWaitingPlayer(ctx context.Context, command EnterMatchmakingCommand) *waitingPlayer {
	return &waitingPlayer{
		command: command,
		result:  make(chan MatchResult, 1),
		done:    ctx.Done(),
	}
}

// findOpponentLocked matches player within its time-control pool or queues it.
// The caller must hold s.mu.
func (s *GameService) findOpponentLocked(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	s.removeCanceledPlayersLocked()

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

// removeCanceledPlayersLocked removes canceled players from every pool.
// The caller must hold s.mu.
func (s *GameService) removeCanceledPlayersLocked() {
	for key, waiting := range s.waitingPlayers {
		if !playerCanceled(waiting.done) {
			continue
		}

		delete(s.waitingPlayers, key)
		close(waiting.result)
	}
}

func (s *GameService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	if player.done == nil {
		return
	}

	<-player.done

	s.mu.Lock()
	defer s.mu.Unlock()

	key := player.command.timeControl
	if s.waitingPlayers[key] != player {
		return
	}

	delete(s.waitingPlayers, key)
	close(player.result)
}

func playerCanceled(done <-chan struct{}) bool {
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

func deliverMatchResult(resultChannel chan MatchResult, result MatchResult) {
	resultChannel <- result
	close(resultChannel)
}
