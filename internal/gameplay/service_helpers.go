package gameplay

import (
	"context"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type waitingPlayer struct {
	command     EnterMatchmakingCommand
	result      chan MatchResult
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
		result:  make(chan MatchResult, 1),
		done:    ctx.Done(),
	}
}

func validateTimeControl(initial, increment time.Duration) error {
	if initial <= 0 || increment < 0 {
		return ErrInvalidTimeControl
	}
	return nil
}

func assignPrivateColors(profileID identity.ProfileID, creatorColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if creatorColor == chess.White {
		return profileID, ""
	}
	return "", profileID
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

func deliverMatchResult(resultChannel chan MatchResult, result MatchResult) {
	resultChannel <- result
	close(resultChannel)
}
