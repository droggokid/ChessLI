package websocket

import (
	"context"
	"errors"
	"testing"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
)

func TestNewSessionInitializesSession(t *testing.T) {
	t.Parallel()

	session := NewSession(identity.NewProfileID(), nil)
	if session.profileID == "" {
		t.Fatal("NewSession() returned an empty profile ID")
	}
	if session.outgoing == nil || cap(session.outgoing) != outgoingBufferSize {
		t.Fatalf("NewSession() outgoing capacity = %d, want %d", cap(session.outgoing), outgoingBufferSize)
	}
	if session.running.Load() {
		t.Fatal("NewSession() returned a running session")
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
