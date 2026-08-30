package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ChessLI/internal/websocket/protocol"
)

func TestNewSessionInitializesSession(t *testing.T) {
	t.Parallel()

	session := NewSession(nil)
	if session.profileID == "" {
		t.Fatal("NewSession() returned an empty profile ID")
	}
	if session.outgoing == nil || cap(session.outgoing) != outgoingBufferSize {
		t.Fatalf("NewSession() outgoing capacity = %d, want %d", cap(session.outgoing), outgoingBufferSize)
	}
	if state := sessionRunState(session.runState.Load()); state != sessionNotStarted {
		t.Fatalf("NewSession() state = %v, want not started", state)
	}
}

func TestSessionRunRejectsMissingHandler(t *testing.T) {
	t.Parallel()

	err := NewSession(nil).Run(context.Background(), nil)
	if err == nil || err.Error() != "websocket message handler is required" {
		t.Fatalf("Run() error = %v, want missing-handler error", err)
	}
}

func TestSessionRunRejectsReusedSession(t *testing.T) {
	t.Parallel()

	for _, state := range []sessionRunState{sessionRunning, sessionStopped} {
		session := NewSession(nil)
		session.runState.Store(uint32(state))

		err := session.Run(context.Background(), func(context.Context, *Session, json.RawMessage) error {
			return nil
		})
		if !errors.Is(err, protocol.ErrSessionAlreadyRun) {
			t.Fatalf("Run() in state %v error = %v, want %v", state, err, protocol.ErrSessionAlreadyRun)
		}
	}
}

func TestSessionSendHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	session := newQueuedSession("player")
	session.outgoing = make(chan protocol.ServerEnvelope)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := session.Send(ctx, protocol.ServerEnvelope{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send() error = %v, want %v", err, context.Canceled)
	}
}
