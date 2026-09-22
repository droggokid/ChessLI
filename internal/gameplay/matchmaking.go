package gameplay

import (
	"context"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
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
		result:  make(chan GameAssignment, 1),
		done:    ctx.Done(),
	}
}

func assignMatchmakingColors(waitingProfileID identity.ProfileID, currentProfileID identity.ProfileID, waitingColor chess.Color) (identity.ProfileID, identity.ProfileID, error) {
	switch waitingColor {
	case chess.White:
		return waitingProfileID, currentProfileID, nil
	case chess.Black:
		return currentProfileID, waitingProfileID, nil
	default:
		return "", "", ErrInvalidColorPreference
	}
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
