package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
)

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

	t.Run("rejects closed session with buffer space", func(t *testing.T) {
		t.Parallel()

		session := newQueuedSession("player")
		session.running.Store(false)
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

func TestFullQueueLogsMetadataWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	client := newQueuedSession("player")
	for len(client.outgoing) < cap(client.outgoing) {
		client.outgoing <- protocol.ServerEnvelope{}
	}
	err := client.Send(context.Background(), protocol.ServerEnvelope{
		Type: protocol.ServerGameState, RequestID: "request", Payload: "private-payload",
	})
	if !errors.Is(err, protocol.ErrSessionQueueFull) {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode queue warning: %v", err)
	}
	if event["event"] != "message.dropped" || event["level"] != "WARN" || event["profile_id"] != "player" ||
		event["request_id"] != "request" || event["message_type"] != string(protocol.ServerGameState) || event["reason"] != "queue_full" {
		t.Fatalf("queue warning = %+v", event)
	}
	if strings.Contains(output.String(), "private-payload") {
		t.Fatal("queue warning included the message payload")
	}
}

func newQueuedSession(profileID identity.ProfileID) *Session {
	session := &Session{
		profileID: profileID,
		outgoing:  make(chan protocol.ServerEnvelope, 8),
	}
	session.running.Store(true)
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
