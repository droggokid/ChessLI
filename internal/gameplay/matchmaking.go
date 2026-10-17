package gameplay

import (
	"context"
	"time"
)

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
