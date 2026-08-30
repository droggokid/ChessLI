package websocket

import (
	"errors"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

func mapApplicationError(err error) (protocol.ErrorCode, string) {
	switch {
	case errors.Is(err, gameplay.ErrGameNotFound):
		return protocol.ErrorGameNotFound, "game not found"
	case errors.Is(err, gameplay.ErrGameFull):
		return protocol.ErrorGameFull, "game is full"
	case errors.Is(err, gameplay.ErrNotParticipant):
		return protocol.ErrorNotPlayer, "not a player in this game"
	case errors.Is(err, gameplay.ErrNotYourTurn):
		return protocol.ErrorNotYourTurn, "not your turn"
	case errors.Is(err, gameplay.ErrIllegalMove):
		return protocol.ErrorIllegalMove, "illegal move"
	case errors.Is(err, gameplay.ErrGameFinished):
		return protocol.ErrorGameFinished, "game is finished"
	case errors.Is(err, gameplay.ErrGameNotReady):
		return protocol.ErrorIllegalMove, "game is waiting for another player"
	case errors.Is(err, gameplay.ErrInvalidColorPreference):
		return protocol.ErrorInvalidMessage, "invalid color preference"
	case errors.Is(err, gameplay.ErrInvalidTimeControl):
		return protocol.ErrorInvalidMessage, "invalid time control"
	case errors.Is(err, gameplay.ErrUnsupportedNotation):
		return protocol.ErrorInvalidMessage, "unsupported move notation"
	case errors.Is(err, gameplay.ErrAlreadyParticipant):
		return protocol.ErrorInvalidMessage, "already a participant in this game"
	case errors.Is(err, gameplay.ErrAlreadyQueued):
		return protocol.ErrorInvalidMessage, "already queued for matchmaking"
	case errors.Is(err, gameplay.ErrNoCompatibleOpponent):
		return protocol.ErrorInvalidMessage, "no compatible opponent available"
	case errors.Is(err, gameplay.ErrStaleGameVersion):
		return protocol.ErrorStaleGameVersion, "game state is stale"
	default:
		return protocol.ErrorInternal, "internal server error"
	}
}
