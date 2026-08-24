package websocket

import (
	"context"
	"errors"
	"testing"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
)

func TestGameSessionsRegistrationLifecycle(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	session := newQueuedSession("player")

	if err := registry.Add("game-1", session); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := registry.Add("game-1", session); err != nil {
		t.Fatalf("idempotent Add() error = %v", err)
	}
	if err := registry.Add("game-2", session); !errors.Is(err, protocol.ErrSessionAlreadyInGame) {
		t.Fatalf("conflicting Add() error = %v, want %v", err, protocol.ErrSessionAlreadyInGame)
	}

	gameID, exists := registry.GameID(session)
	if !exists || gameID != "game-1" {
		t.Fatalf("GameID() = (%q, %v), want (%q, true)", gameID, exists, "game-1")
	}

	registry.Remove(session)
	if gameID, exists = registry.GameID(session); exists || gameID != "" {
		t.Fatalf("GameID() after Remove = (%q, %v), want empty and false", gameID, exists)
	}
	if len(registry.byGame) != 0 {
		t.Fatalf("byGame size after Remove = %d, want 0", len(registry.byGame))
	}
}

func TestGameSessionsBroadcastPreservesOnlySourceRequestID(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	source := newQueuedSession("source")
	peer := newQueuedSession("peer")
	if err := registry.Add("game", source); err != nil {
		t.Fatalf("Add(source) error = %v", err)
	}
	if err := registry.Add("game", peer); err != nil {
		t.Fatalf("Add(peer) error = %v", err)
	}

	message := protocol.ServerEnvelope{
		Type:      protocol.ServerGameState,
		RequestID: "request",
		Payload:   protocol.GameStatePayload{GameID: "game", Version: 1},
	}
	if err := registry.Broadcast(context.Background(), "game", source, message); err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}

	sourceMessage := receiveEnvelope(t, source)
	peerMessage := receiveEnvelope(t, peer)
	if sourceMessage.RequestID != "request" {
		t.Fatalf("source RequestID = %q, want %q", sourceMessage.RequestID, "request")
	}
	if peerMessage.RequestID != "" {
		t.Fatalf("peer RequestID = %q, want empty", peerMessage.RequestID)
	}
}

func TestGameSessionsBroadcastReturnsSourceFailure(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	source := newQueuedSession("source")
	peer := newQueuedSession("peer")
	source.outgoing = make(chan protocol.ServerEnvelope)
	close(source.done)
	if err := registry.Add("game", source); err != nil {
		t.Fatalf("Add(source) error = %v", err)
	}
	if err := registry.Add("game", peer); err != nil {
		t.Fatalf("Add(peer) error = %v", err)
	}

	err := registry.Broadcast(context.Background(), "game", source, protocol.ServerEnvelope{})
	if !errors.Is(err, protocol.ErrSessionNotRunning) {
		t.Fatalf("Broadcast() error = %v, want %v", err, protocol.ErrSessionNotRunning)
	}
	if len(peer.outgoing) != 0 {
		t.Fatalf("peer received %d messages after source failure, want 0", len(peer.outgoing))
	}
}

func TestSessionSend(t *testing.T) {
	t.Parallel()

	t.Run("rejects session that has not started", func(t *testing.T) {
		t.Parallel()

		session := &Session{}
		err := session.Send(context.Background(), protocol.ServerEnvelope{})
		if !errors.Is(err, protocol.ErrSessionNotRunning) {
			t.Fatalf("Send() error = %v, want %v", err, protocol.ErrSessionNotRunning)
		}
	})

	t.Run("queues message", func(t *testing.T) {
		t.Parallel()

		session := newQueuedSession("player")
		want := protocol.ServerEnvelope{Type: protocol.ServerConnectionReady}
		if err := session.Send(context.Background(), want); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if got := receiveEnvelope(t, session); got.Type != want.Type {
			t.Fatalf("queued type = %q, want %q", got.Type, want.Type)
		}
	})

	t.Run("rejects closed session", func(t *testing.T) {
		t.Parallel()

		session := newQueuedSession("player")
		session.outgoing = make(chan protocol.ServerEnvelope)
		close(session.done)
		err := session.Send(context.Background(), protocol.ServerEnvelope{})
		if !errors.Is(err, protocol.ErrSessionNotRunning) {
			t.Fatalf("Send() error = %v, want %v", err, protocol.ErrSessionNotRunning)
		}
	})
}

func newQueuedSession(profileID identity.ProfileID) *Session {
	session := &Session{
		profileID: profileID,
		outgoing:  make(chan protocol.ServerEnvelope, 8),
		done:      make(chan struct{}),
	}
	session.started.Store(true)
	return session
}

func receiveEnvelope(t *testing.T, session *Session) protocol.ServerEnvelope {
	t.Helper()

	select {
	case message := <-session.outgoing:
		return message
	default:
		t.Fatal("session has no queued message")
		return protocol.ServerEnvelope{}
	}
}
