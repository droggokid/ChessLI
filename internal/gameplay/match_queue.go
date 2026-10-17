package gameplay

import (
	"context"
	"log/slog"
	"time"
)

// matchQueue is owned by GameService; every access requires the service mutex.
type matchQueue map[timeControlKey]*waitingPlayer

func (q matchQueue) findOpponent(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	q.removeCanceledPlayers()
	for _, waiting := range q {
		if waiting.command.ProfileID == player.command.ProfileID {
			return nil, false, ErrAlreadyQueued
		}
	}

	key := timeControlKey{initial: player.command.Initial, increment: player.command.Increment}
	waiting, exists := q[key]
	if !exists {
		q[key] = player
		return nil, true, nil
	}

	delete(q, key)
	stopWaitingPlayerCleanup(waiting)
	return waiting, false, nil
}

func (q matchQueue) removeCanceledPlayers() {
	for key, waiting := range q {
		if !playerCanceled(waiting.done) {
			continue
		}
		delete(q, key)
		stopWaitingPlayerCleanup(waiting)
		slog.Info("matchmaking canceled", "event", "matchmaking.canceled", "profile_id", waiting.command.ProfileID)
		close(waiting.result)
	}
}

func (q matchQueue) removeCanceledPlayer(player *waitingPlayer) {
	key := timeControlKey{initial: player.command.Initial, increment: player.command.Increment}
	// A stopped cleanup callback may already be running after this player was matched.
	if q[key] != player {
		return
	}
	delete(q, key)
	player.stopCleanup = nil
	slog.Info("matchmaking canceled", "event", "matchmaking.canceled", "profile_id", player.command.ProfileID)
	close(player.result)
}

type waitingPlayer struct {
	command     EnterMatchmakingCommand
	result      chan GameAssignment
	done        <-chan struct{}
	stopCleanup func() bool
}

// timeControlKey identifies one matchmaking pool by its clock settings.
type timeControlKey struct {
	initial   time.Duration
	increment time.Duration
}

func newWaitingPlayer(ctx context.Context, command EnterMatchmakingCommand) *waitingPlayer {
	return &waitingPlayer{
		command: command,
		result:  make(chan GameAssignment, 1), // Matching never waits for the receiver.
		done:    ctx.Done(),
	}
}

func playerCanceled(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func stopWaitingPlayerCleanup(player *waitingPlayer) {
	if player.stopCleanup == nil {
		return
	}

	player.stopCleanup()
	player.stopCleanup = nil
}

func deliverMatchResult(resultChannel chan GameAssignment, result GameAssignment) {
	resultChannel <- result
	close(resultChannel)
}
