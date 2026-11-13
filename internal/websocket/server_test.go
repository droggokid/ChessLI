package websocket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/persistence"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"go.uber.org/mock/gomock"
)

func newTestGameService(t *testing.T) *gameplay.GameService {
	t.Helper()
	repository := persistence.NewMockGameRepository(gomock.NewController(t))
	repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	profiles := persistence.NewMockProfileRepository(gomock.NewController(t))
	profiles.EXPECT().GetProfileByID(gomock.Any(), gomock.Any()).Return(persistence.Profile{}, nil).AnyTimes()
	service := gameplay.NewGameService(repository, profiles)
	t.Cleanup(service.Close)
	return service
}

func TestServerRejectsInvalidConnectionProfiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		query  string
		err    error
		status int
	}{
		{name: "missing ID", status: http.StatusBadRequest},
		{name: "invalid ID", query: "?profileId=invalid", status: http.StatusBadRequest},
		{name: "unknown profile", query: "?profileId=00000000-0000-4000-8000-000000000001", err: gameplay.ErrProfileNotFound, status: http.StatusNotFound},
		{name: "database unavailable", query: "?profileId=00000000-0000-4000-8000-000000000001", err: errors.New("database unavailable"), status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			games := gameplay.NewMockGameService(gomock.NewController(t))
			if test.err != nil {
				games.EXPECT().ValidateProfile(gomock.Any(), gomock.Any()).Return(test.err)
			}
			server := NewServer("", games)
			t.Cleanup(server.cancelConnections)
			response := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ws"+test.query, nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestNewServerWiresDependencies(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:0", nil)
	if server.httpServer == nil || server.httpServer.Addr != "127.0.0.1:0" {
		t.Fatalf("NewServer() HTTP address = %q, want %q", server.httpServer.Addr, "127.0.0.1:0")
	}
	if server.messageHandler == nil || server.messageHandler.gameSessions == nil {
		t.Fatal("NewServer() did not initialize handler dependencies")
	}
}

func TestServerBroadcastGameState(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:0", nil)
	client := newQueuedSession("white")

	if err := server.messageHandler.gameSessions.Add("game", client); err != nil {
		t.Fatalf("gameSessions.Add() error = %v", err)
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

func TestServerBroadcastTerminalStateReleasesSessions(t *testing.T) {
	t.Parallel()

	server := NewServer("127.0.0.1:0", nil)
	white := newQueuedSession("white")
	black := newQueuedSession("black")
	for _, client := range []*Session{white, black} {
		if err := server.messageHandler.gameSessions.Add("game", client); err != nil {
			t.Fatalf("gameSessions.Add() error = %v", err)
		}
	}

	server.BroadcastGameState(gameplay.GameSnapshot{
		GameID:  "game",
		Outcome: game.OutcomeWhiteWin,
	})

	for _, client := range []*Session{white, black} {
		if err := server.messageHandler.gameSessions.Hold(client); err != nil {
			t.Fatalf("Hold() after terminal broadcast error = %v", err)
		}
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
