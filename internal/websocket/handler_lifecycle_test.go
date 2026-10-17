package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"testing/synctest"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	"github.com/corentings/chess/v2"
	"go.uber.org/mock/gomock"
)

func TestHandlerCloseWaitsForMatchmakingCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		games := gameplay.NewGameService()
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

func TestHandlerCloseJoinsMatchAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := gameplay.NewMockGameService(gomock.NewController(t))
		service.EXPECT().EnterMatchmaking(gomock.Any(), gomock.Any()).Return(gameplay.MatchTicket{Result: matchResults(gameplay.GameAssignment{GameID: "game", Color: chess.White})}, nil)
		started, release := make(chan struct{}), make(chan struct{})
		service.EXPECT().GameState(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, identity.GameID) (gameplay.GameSnapshot, error) {
			close(started)
			<-release
			return gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", BlackProfileID: "black"}, nil
		})
		handler := newHandler(service, newGameSessions())
		client := newQueuedSession("white")
		if err := handler.Handle(context.Background(), client, json.RawMessage(`{"type":"matchmaking.enter","payload":{"timeControl":"1+0"}}`)); err != nil {
			t.Fatal(err)
		}
		<-started
		closed := make(chan struct{})
		go func() { handler.Close(); close(closed) }()
		synctest.Wait()
		select {
		case <-closed:
			t.Error("Close returned before matchmaking admission")
		default:
		}
		close(release)
		<-closed
		receiveEnvelope(t, client) // matchmaking.entered
		receiveEnvelope(t, client) // matchmaking.found
		if got := receiveEnvelope(t, client); got.Type != protocol.ServerGameInitial {
			t.Fatalf("last response = %+v", got)
		}
	})
}
