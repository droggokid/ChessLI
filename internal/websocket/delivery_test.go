package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"ChessLI/internal/game"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
	"go.uber.org/mock/gomock"
)

func TestHandlerAutomaticallyPublishesTimeoutAndReleasesSessions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		games := gameplay.NewGameService()
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
			if err := server.gameSessions.Hold(session); err != nil {
				t.Fatalf("timeout did not release session: %v", err)
			}
		}
	})
}

func TestHandlerTerminalStateReachesPeerWhenSourceStops(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	registry := NewGameSessions()
	source, peer := newQueuedSession("white"), newQueuedSession("black")
	for _, session := range []*Session{source, peer} {
		if err := registry.Add("game", session); err != nil {
			t.Fatal(err)
		}
	}
	service.EXPECT().Resign(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, gameplay.ResignCommand) (gameplay.GameSnapshot, error) {
			source.runState.Store(uint32(sessionStopped))
			return gameplay.GameSnapshot{GameID: "game", Outcome: game.OutcomeBlackWin, Termination: game.TerminationResignation}, nil
		})
	err := NewHandler(service, registry).Handle(context.Background(), source,
		json.RawMessage(`{"type":"game.resign","requestId":"resign","payload":{"gameId":"game"}}`))
	if !errors.Is(err, protocol.ErrSessionNotRunning) {
		t.Fatalf("Handle() error = %v, want stopped session", err)
	}
	message := receiveEnvelope(t, peer)
	if message.RequestID != "" || message.Payload.(protocol.GameStatePayload).Status != protocol.GameStatusFinished {
		t.Fatalf("peer response = %+v, want unsolicited terminal state", message)
	}
	for _, session := range []*Session{source, peer} {
		if err := registry.Hold(session); err != nil {
			t.Fatalf("terminal broadcast did not release session: %v", err)
		}
	}
}

func TestAwaitMatchFinishedGameIsReadOnly(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	state := gameplay.GameSnapshot{
		GameID: "game", WhiteProfileID: "white", BlackProfileID: "black",
		Outcome: game.OutcomeBlackWin, Termination: game.TerminationResignation,
	}
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(state, nil)
	registry := NewGameSessions()
	client, peer := newQueuedSession("white"), newQueuedSession("black")
	if err := registry.Hold(client); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add("game", peer); err != nil {
		t.Fatal(err)
	}
	NewHandler(service, registry).awaitMatch(context.Background(), client, "match", gameplay.MatchTicket{
		Result: matchResults(gameplay.GameAssignment{GameID: "game", Color: chess.White}),
	})
	if got := receiveEnvelope(t, client); got.Type != protocol.ServerMatchFound {
		t.Fatalf("first response = %+v", got)
	}
	initial := receiveEnvelope(t, client)
	payload := initial.Payload.(protocol.GameStatePayload)
	if initial.Type != protocol.ServerGameInitial || payload.Status != protocol.GameStatusFinished ||
		payload.White.Connected || payload.Black.Connected {
		t.Fatalf("late admission state = %+v, want read-only finished game", payload)
	}
	if got := receiveEnvelope(t, peer); got.Payload.(protocol.GameStatePayload).Status != protocol.GameStatusFinished {
		t.Fatalf("peer did not receive terminal state: %+v", got)
	}
	for _, session := range []*Session{client, peer} {
		if err := registry.Hold(session); err != nil {
			t.Fatalf("late admission occupied finished game: %v", err)
		}
	}
}

func TestHandlerQueuesStateBeforeConcurrentTimeoutPublication(t *testing.T) {
	for _, action := range []string{"move", "resume"} {
		t.Run(action, func(t *testing.T) {
			service := gameplay.NewMockGameService(gomock.NewController(t))
			server := NewServer("", service)
			t.Cleanup(server.cancelConnections)
			client, peer := newQueuedSession("white"), newQueuedSession("black")
			if err := server.gameSessions.Add("game", peer); err != nil {
				t.Fatal(err)
			}
			active := gameplay.GameSnapshot{GameID: "game", Version: 1, WhiteProfileID: "white", BlackProfileID: "black"}
			terminal := active
			terminal.Version = 2
			terminal.Outcome = game.OutcomeBlackWin
			terminal.Termination = game.TerminationTimeout
			started, published := make(chan struct{}), make(chan struct{})
			// Expiration occurs while an operation is returning an earlier snapshot.
			expire := func() (gameplay.GameSnapshot, error) {
				go func() {
					close(started)
					server.BroadcastGameState(terminal)
					close(published)
				}()
				<-started
				return active, nil
			}
			var raw json.RawMessage
			if action == "move" {
				if err := server.gameSessions.Add("game", client); err != nil {
					t.Fatal(err)
				}
				service.EXPECT().MakeMove(gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, gameplay.MoveCommand) (gameplay.GameSnapshot, error) { return expire() })
				raw = json.RawMessage(`{"type":"game.move","payload":{"gameId":"game","move":"e2e4","expectedVersion":0}}`)
			} else {
				service.EXPECT().ResumeGame(gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, gameplay.ResumeGameCommand) (gameplay.GameSnapshot, error) { return expire() })
				raw = json.RawMessage(`{"type":"game.resume","payload":{"gameId":"game"}}`)
			}
			if err := server.messageHandler.Handle(context.Background(), client, raw); err != nil {
				t.Fatal(err)
			}
			select {
			case <-published:
			case <-time.After(time.Second):
				t.Fatal("timeout publication did not finish")
			}
			for _, session := range []*Session{client, peer} {
				finished := false
				for len(session.outgoing) != 0 {
					message := receiveEnvelope(t, session)
					state, ok := message.Payload.(protocol.GameStatePayload)
					if !ok {
						continue
					}
					if finished && state.Status != protocol.GameStatusFinished {
						t.Fatalf("active state followed terminal state: %+v", state)
					}
					finished = state.Status == protocol.GameStatusFinished
				}
				if !finished {
					t.Fatal("session did not receive terminal state")
				}
				if err := server.gameSessions.Hold(session); err != nil {
					t.Fatalf("session remained in finished game: %v", err)
				}
			}
		})
	}
}
