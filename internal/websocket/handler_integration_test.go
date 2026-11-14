package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
)

func TestIntegrationHandlerDisconnectCancelsMatchmaking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		games := newTestGameService(t)
		defer games.Close()
		registry := newGameSessions()
		handler := newHandler(games, registry)
		client := newQueuedSession("player")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := handler.Handle(ctx, client, json.RawMessage(`{"type":"matchmaking.enter","payload":{"timeControl":"1+0"}}`)); err != nil {
			t.Fatal(err)
		}
		handler.Disconnect(t.Context(), client)
		if len(handler.pending) != 0 {
			t.Fatal("disconnect retained a pending entry")
		}
		if _, err := games.CreatePrivateGame(t.Context(), gameplay.CreatePrivateCommand{ProfileID: client.profileID, Initial: time.Minute}); err != nil {
			t.Fatalf("disconnected profile remained queued: %v", err)
		}
		if err := registry.Hold(client); err != nil {
			t.Fatalf("matchmaking left session reserved: %v", err)
		}
	})
}

func TestIntegrationHandlerLateDisconnectPreservesNewMatchmakingEntry(t *testing.T) {
	games := newTestGameService(t)
	t.Cleanup(games.Close)
	handler := newHandler(games, newGameSessions())
	old, replacement, peer := newQueuedSession("player"), newQueuedSession("player"), newQueuedSession("peer")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	raw := json.RawMessage(`{"type":"matchmaking.enter","requestId":"match","payload":{"timeControl":"1+0"}}`)
	if err := handler.Handle(ctx, old, raw); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := handler.Handle(t.Context(), replacement, raw); err != nil {
		t.Fatal(err)
	}
	handler.Disconnect(t.Context(), old)
	if err := handler.Handle(t.Context(), peer, raw); err != nil {
		t.Fatal(err)
	}
	for _, want := range []protocol.ServerMessageType{protocol.ServerMatchmakingEntered, protocol.ServerMatchFound, protocol.ServerGameInitial} {
		if got := receiveEnvelope(t, replacement); got.Type != want {
			t.Fatalf("replacement response = %s, want %s", got.Type, want)
		}
	}
	if len(handler.pending) != 0 || handler.gameSessions.IsBusy(old) || games.ActiveGame("player") == "" {
		t.Fatal("late disconnect disturbed replacement admission")
	}
}

func TestIntegrationMatchmakingPublishesPeerWhenSourceQueueIsFull(t *testing.T) {
	games := newTestGameService(t)
	t.Cleanup(games.Close)
	handler := newHandler(games, newGameSessions())
	peer, source := newQueuedSession("peer"), newQueuedSession("source")
	raw := json.RawMessage(`{"type":"matchmaking.enter","requestId":"match","payload":{"timeControl":"1+0"}}`)
	if err := handler.Handle(t.Context(), peer, raw); err != nil {
		t.Fatal(err)
	}
	for len(source.outgoing) < cap(source.outgoing) {
		source.outgoing <- protocol.ServerEnvelope{}
	}
	if err := handler.Handle(t.Context(), source, raw); !errors.Is(err, protocol.ErrSessionQueueFull) {
		t.Fatalf("admission error = %v, want queue full", err)
	}
	for _, want := range []protocol.ServerMessageType{protocol.ServerMatchmakingEntered, protocol.ServerMatchFound, protocol.ServerGameInitial} {
		if got := receiveEnvelope(t, peer); got.Type != want {
			t.Fatalf("peer response = %s, want %s", got.Type, want)
		}
	}
	gameID := games.ActiveGame("source")
	if gameID == "" || gameID != games.ActiveGame("peer") || handler.gameSessions.IsBusy(source) || len(handler.pending) != 0 {
		t.Fatal("failed acknowledgement lost the committed match or retained a reservation")
	}
	if _, err := games.ResumeGame(t.Context(), gameplay.ResumeGameCommand{GameID: gameID, ProfileID: source.profileID}); err != nil {
		t.Fatalf("source could not recover the committed game: %v", err)
	}
}

func TestIntegrationHandlerAutomaticallyPublishesTimeoutAndReleasesSessions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		games := newTestGameService(t)
		defer games.Close()
		server := NewServer("", games)
		defer server.cancelConnections()
		games.SetGameExpiredHandler(server.BroadcastGameState)
		white, black := newQueuedSession("white"), newQueuedSession("black")
		if err := server.messageHandler.Handle(context.Background(), white,
			json.RawMessage(`{"type":"game.create","payload":{"color":"white","timeControl":"1+0"}}`)); err != nil {
			t.Fatal(err)
		}
		created := receiveEnvelope(t, white).Payload.(protocol.GameCreatedPayload)
		if err := server.messageHandler.Handle(context.Background(), black,
			json.RawMessage(`{"type":"game.join","payload":{"gameId":"`+string(created.GameID)+`"}}`)); err != nil {
			t.Fatal(err)
		}
		receiveEnvelope(t, black) // game.joined
		receiveEnvelope(t, black) // game.initial
		receiveEnvelope(t, white) // game.initial

		// Advance the synctest clock to the actual game deadline.
		<-time.After(time.Minute)
		synctest.Wait()
		for _, session := range []*Session{white, black} {
			message := receiveEnvelope(t, session)
			state := message.Payload.(protocol.GameStatePayload)
			if message.Type != protocol.ServerGameState || state.Version != 1 ||
				state.Outcome == nil || state.Outcome.Reason != protocol.GameOverTimeout {
				t.Fatalf("automatic timeout = %+v", message)
			}
			if len(session.outgoing) != 0 {
				t.Fatal("timeout published more than once")
			}
			if err := server.messageHandler.gameSessions.Hold(session); err != nil {
				t.Fatalf("timeout did not release session: %v", err)
			}
		}
	})
}

func TestIntegrationAdmissionPublishesPresenceWhenSourceQueueIsFull(t *testing.T) {
	for _, action := range []string{"join", "resume", "match"} {
		t.Run(action, func(t *testing.T) {
			games := newTestGameService(t)
			t.Cleanup(games.Close)
			ctx := context.Background()
			created, err := games.CreatePrivateGame(ctx, gameplay.CreatePrivateCommand{
				ProfileID: "white", Initial: time.Minute, ColorPreference: gameplay.ColorWhite,
			})
			if err != nil {
				t.Fatal(err)
			}
			if action != "join" {
				if _, err := games.JoinPrivateGame(ctx, gameplay.JoinPrivateCommand{GameID: created.GameID, ProfileID: "black"}); err != nil {
					t.Fatal(err)
				}
			}
			registry := newGameSessions()
			client, peer := newQueuedSession("black"), newQueuedSession("white")
			if err := registry.Add(created.GameID, peer); err != nil {
				t.Fatal(err)
			}
			for len(client.outgoing) < cap(client.outgoing) {
				client.outgoing <- protocol.ServerEnvelope{}
			}
			handler := newHandler(games, registry)
			if action == "match" {
				if err := registry.Hold(client); err != nil {
					t.Fatal(err)
				}
				handler.admitMatch(ctx, client, "admit", created.GameID)
			} else {
				raw := json.RawMessage(`{"type":"game.` + action + `","requestId":"admit","payload":{"gameId":"` + string(created.GameID) + `"}}`)
				if err := handler.Handle(ctx, client, raw); !errors.Is(err, protocol.ErrSessionQueueFull) {
					t.Fatalf("admission error = %v, want queue full", err)
				}
			}
			message := receiveEnvelope(t, peer)
			state := message.Payload.(protocol.GameStatePayload)
			if message.RequestID != "" || state.Black == nil || !state.Black.Connected {
				t.Fatalf("peer did not receive committed presence: %+v", message)
			}
		})
	}
}
