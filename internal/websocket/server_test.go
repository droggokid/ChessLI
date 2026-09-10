package websocket

import (
	"context"
	"errors"
	"testing"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
)

func TestNewServerWiresDependencies(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:0", nil)
	if server.httpServer == nil || server.httpServer.Addr != "127.0.0.1:0" {
		t.Fatalf("NewServer() HTTP address = %q, want %q", server.httpServer.Addr, "127.0.0.1:0")
	}
	if server.gameSessions == nil || server.messageHandler == nil {
		t.Fatal("NewServer() did not initialize handler dependencies")
	}
	if server.messageHandler.gameSessions != server.gameSessions {
		t.Fatal("handler and server do not share the same session registry")
	}
}

func TestServerBroadcastGameState(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:0", nil)
	client := newQueuedSession("white")

	if err := server.gameSessions.Add("game", client); err != nil {
		t.Fatalf("GameSessions.Add() error = %v", err)
	}

	server.BroadcastGameState(gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		Version:        1,
		WhiteProfileID: "white",
	})

	message := receiveEnvelope(t, client)
	payload, ok := message.Payload.(protocol.GameStatePayload)
	if message.Type != protocol.ServerGameState || !ok {
		t.Fatalf("broadcast envelope = %+v, want game.state", message)
	}
	if payload.GameID != "game" || payload.Version != 1 || payload.FEN != "fen" {
		t.Fatalf("broadcast payload = %+v, want authoritative state", payload)
	}
}

func TestIsExpectedClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "canceled", err: context.Canceled, want: true},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "normal closure", err: coderws.CloseError{Code: coderws.StatusNormalClosure}, want: true},
		{name: "going away", err: coderws.CloseError{Code: coderws.StatusGoingAway}, want: true},
		{name: "unexpected", err: errors.New("network failure")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isExpectedClose(tt.err); got != tt.want {
				t.Fatalf("isExpectedClose() = %v, want %v", got, tt.want)
			}
		})
	}
}
