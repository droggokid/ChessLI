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

	"github.com/corentings/chess/v2"
)

func TestIntegrationHandlerCloseWaitsForMatchmakingCleanup(t *testing.T) {
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
		cancel()
		handler.Close()
		if err := registry.Hold(client); err != nil {
			t.Fatalf("matchmaking left session reserved: %v", err)
		}
	})
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
				handler.awaitMatch(ctx, client, "admit", gameplay.MatchTicket{Result: matchResults(gameplay.GameAssignment{GameID: created.GameID, Color: chess.Black})})
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
