package websocket

import (
	"errors"
	"testing"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

func TestMapApplicationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err         error
		wantCode    protocol.ErrorCode
		wantMessage string
	}{
		{gameplay.ErrGameNotFound, protocol.ErrorGameNotFound, "game not found"},
		{gameplay.ErrGameFull, protocol.ErrorGameFull, "game is full"},
		{gameplay.ErrNotParticipant, protocol.ErrorNotPlayer, "not a player in this game"},
		{gameplay.ErrNotYourTurn, protocol.ErrorNotYourTurn, "not your turn"},
		{gameplay.ErrIllegalMove, protocol.ErrorIllegalMove, "illegal move"},
		{gameplay.ErrGameFinished, protocol.ErrorGameFinished, "game is finished"},
		{gameplay.ErrGameNotReady, protocol.ErrorIllegalMove, "game is waiting for another player"},
		{gameplay.ErrInvalidColorPreference, protocol.ErrorInvalidMessage, "invalid color preference"},
		{gameplay.ErrInvalidTimeControl, protocol.ErrorInvalidMessage, "invalid time control"},
		{gameplay.ErrUnsupportedNotation, protocol.ErrorInvalidMessage, "unsupported move notation"},
		{gameplay.ErrAlreadyParticipant, protocol.ErrorInvalidMessage, "already a participant in this game"},
		{gameplay.ErrAlreadyQueued, protocol.ErrorInvalidMessage, "already queued for matchmaking"},
		{gameplay.ErrNoCompatibleOpponent, protocol.ErrorInvalidMessage, "no compatible opponent available"},
		{gameplay.ErrStaleGameVersion, protocol.ErrorStaleGameVersion, "game state is stale"},
		{errors.New("unexpected"), protocol.ErrorInternal, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.wantCode)+"_"+tt.wantMessage, func(t *testing.T) {
			t.Parallel()

			code, message := mapApplicationError(tt.err)
			if code != tt.wantCode || message != tt.wantMessage {
				t.Fatalf("mapApplicationError() = (%q, %q), want (%q, %q)", code, message, tt.wantCode, tt.wantMessage)
			}
		})
	}
}
