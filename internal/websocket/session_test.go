package websocket

import (
	"context"
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
	if session.done == nil {
		t.Fatal("NewSession() did not initialize done channel")
	}
	if session.started.Load() {
		t.Fatal("NewSession() is already marked as started")
	}
}

func TestSessionRunRejectsMissingHandler(t *testing.T) {
	t.Parallel()

	err := NewSession(nil).Run(context.Background(), nil)
	if err == nil || err.Error() != "websocket message handler is required" {
		t.Fatalf("Run() error = %v, want missing-handler error", err)
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
