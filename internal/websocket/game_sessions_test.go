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

	if !registry.IsConnected("game-1", "player") {
		t.Fatal("IsConnected() = false, want registered session")
	}

	if gameID := registry.Remove(session); gameID != "game-1" {
		t.Fatalf("Remove() game ID = %q, want game-1", gameID)
	}
	if registry.IsConnected("game-1", "player") {
		t.Fatal("IsConnected() = true after Remove")
	}
	if len(registry.byGame) != 0 {
		t.Fatalf("byGame size after Remove = %d, want 0", len(registry.byGame))
	}
}

func TestGameSessionsReservationPreventsConflictingLifecycle(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	session := newQueuedSession("player")
	if err := registry.Hold(session); err != nil {
		t.Fatalf("Hold() error = %v", err)
	}
	if err := registry.Hold(session); !errors.Is(err, protocol.ErrSessionAlreadyInGame) {
		t.Fatalf("Hold() error = %v, want %v", err, protocol.ErrSessionAlreadyInGame)
	}
	if registry.IsConnected("game", "player") {
		t.Fatal("IsConnected() reports held session as in-game")
	}
	if err := registry.Add("game", session); err != nil {
		t.Fatalf("Add() queued session error = %v", err)
	}
	if !registry.IsConnected("game", "player") {
		t.Fatal("IsConnected() = false after Add")
	}
}

func TestGameSessionsIsConnected(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	session := newQueuedSession("player")
	if err := registry.Add("game", session); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if !registry.IsConnected("game", "player") {
		t.Fatal("IsConnected() = false, want true for registered profile")
	}
	if registry.IsConnected("game", "other") {
		t.Fatal("IsConnected() = true, want false for different profile")
	}
	if registry.IsConnected("other-game", "player") {
		t.Fatal("IsConnected() = true, want false for different game")
	}

	registry.Remove(session)
	if registry.IsConnected("game", "player") {
		t.Fatal("IsConnected() = true after Remove, want false")
	}
}

func TestGameSessionsRejectsDuplicateProfileInGame(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	if err := registry.Add("game", newQueuedSession("player")); err != nil {
		t.Fatalf("Add() first session error = %v", err)
	}

	err := registry.Add("game", newQueuedSession("player"))
	if !errors.Is(err, protocol.ErrSessionAlreadyInGame) {
		t.Fatalf("Add() duplicate profile error = %v, want %v", err, protocol.ErrSessionAlreadyInGame)
	}
}

func TestGameSessionsRejectsStoppedSession(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	session := newQueuedSession("player")
	session.runState.Store(uint32(sessionStopped))

	err := registry.Add("game", session)
	if !errors.Is(err, protocol.ErrSessionNotRunning) {
		t.Fatalf("Add() error = %v, want %v", err, protocol.ErrSessionNotRunning)
	}
}

func TestGameSessionsRemoveGameReleasesAllSessions(t *testing.T) {
	t.Parallel()

	registry := NewGameSessions()
	white := newQueuedSession("white")
	black := newQueuedSession("black")
	for _, session := range []*Session{white, black} {
		if err := registry.Add("game", session); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	registry.RemoveGame("game")

	for _, session := range []*Session{white, black} {
		if err := registry.Hold(session); err != nil {
			t.Fatalf("Hold() after RemoveGame() error = %v", err)
		}
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

func TestGameSessionsBroadcastAttemptsPeersAfterSourceFailure(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"stopped", "full", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			registry := NewGameSessions()
			source, peer := newQueuedSession("source"), newQueuedSession("peer")
			for _, session := range []*Session{source, peer} {
				if err := registry.Add("game", session); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wantErr error
			switch failure {
			case "stopped":
				source.runState.Store(uint32(sessionStopped))
				wantErr = protocol.ErrSessionNotRunning
			case "full":
				for len(source.outgoing) < cap(source.outgoing) {
					source.outgoing <- protocol.ServerEnvelope{}
				}
				wantErr = protocol.ErrSessionQueueFull
			case "canceled":
				cancel()
				wantErr = context.Canceled
			}
			message := protocol.ServerEnvelope{Type: protocol.ServerGameState, RequestID: "source-request"}
			if err := registry.Broadcast(ctx, "game", source, message); !errors.Is(err, wantErr) {
				t.Fatalf("Broadcast() error = %v, want %v", err, wantErr)
			}
			if got := receiveEnvelope(t, peer); got.Type != message.Type || got.RequestID != "" {
				t.Fatalf("peer envelope = %+v, want unsolicited state", got)
			}
		})
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
		session.runState.Store(uint32(sessionStopped))
		err := session.Send(context.Background(), protocol.ServerEnvelope{})
		if !errors.Is(err, protocol.ErrSessionNotRunning) {
			t.Fatalf("Send() error = %v, want %v", err, protocol.ErrSessionNotRunning)
		}
	})

	t.Run("rejects closed session with buffer space", func(t *testing.T) {
		t.Parallel()

		session := newQueuedSession("player")
		session.runState.Store(uint32(sessionStopped))
		err := session.Send(context.Background(), protocol.ServerEnvelope{})
		if !errors.Is(err, protocol.ErrSessionNotRunning) {
			t.Fatalf("Send() error = %v, want %v", err, protocol.ErrSessionNotRunning)
		}
		if len(session.outgoing) != 0 {
			t.Fatalf("Send() queued %d messages after close, want 0", len(session.outgoing))
		}
	})

	t.Run("rejects full outgoing queue", func(t *testing.T) {
		t.Parallel()

		session := newQueuedSession("player")
		session.outgoing = make(chan protocol.ServerEnvelope, 1)
		session.outgoing <- protocol.ServerEnvelope{Type: protocol.ServerConnectionReady}

		err := session.Send(context.Background(), protocol.ServerEnvelope{})
		if !errors.Is(err, protocol.ErrSessionQueueFull) {
			t.Fatalf("Send() error = %v, want %v", err, protocol.ErrSessionQueueFull)
		}
		if len(session.outgoing) != 1 {
			t.Fatalf("outgoing queue length = %d, want 1", len(session.outgoing))
		}
	})
}

func newQueuedSession(profileID identity.ProfileID) *Session {
	session := &Session{
		profile:  identity.Profile{ID: profileID},
		outgoing: make(chan protocol.ServerEnvelope, 8),
	}
	session.runState.Store(uint32(sessionRunning))
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
