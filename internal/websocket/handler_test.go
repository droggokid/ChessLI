package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
	"go.uber.org/mock/gomock"
)

func TestHandlerProtocolErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		raw           json.RawMessage
		wantRequestID string
		wantCode      protocol.ErrorCode
	}{
		{name: "invalid JSON", raw: json.RawMessage(`{"type":`), wantCode: protocol.ErrorInvalidMessage},
		{name: "unknown type", raw: json.RawMessage(`{"type":"unknown","requestId":"request"}`), wantRequestID: "request", wantCode: protocol.ErrorUnknownType},
		{name: "unknown envelope field is ignored", raw: json.RawMessage(`{"type":"game.resign","requestId":"request","future":true}`), wantRequestID: "request", wantCode: protocol.ErrorNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newQueuedSession("player")
			service := NewMockGameService(gomock.NewController(t))
			handler := NewHandler(service, NewGameSessions())
			if err := handler.Handle(context.Background(), client, tt.raw); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			message := receiveEnvelope(t, client)
			if message.Type != protocol.ServerError || message.RequestID != tt.wantRequestID {
				t.Fatalf("error envelope = %+v, want request ID %q", message, tt.wantRequestID)
			}
			payload, ok := message.Payload.(protocol.ErrorPayload)
			if !ok || payload.Code != tt.wantCode {
				t.Fatalf("error payload = %+v, want code %q", message.Payload, tt.wantCode)
			}
		})
	}
}

func TestHandlerCreateGameMapsAndRegisters(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().CreatePrivateGame(gomock.Any(), gameplay.CreatePrivateCommand{
		ProfileID:       "player",
		Initial:         3 * time.Minute,
		Increment:       2 * time.Second,
		ColorPreference: gameplay.ColorBlack,
	}).Return(gameplay.CreateResult{GameID: "game", Color: chess.Black}, nil)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	raw := json.RawMessage(`{"type":"game.create","requestId":"request","payload":{"timeControl":"3+2","color":"black"}}`)

	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	message := receiveEnvelope(t, client)
	payload, ok := message.Payload.(protocol.GameCreatedPayload)
	if message.Type != protocol.ServerGameCreated || message.RequestID != "request" || !ok || payload.GameID != "game" || payload.Color != protocol.ColorBlack {
		t.Fatalf("created envelope = %+v, want black game.created response", message)
	}
	if gameID, exists := registry.GameID(client); !exists || gameID != "game" {
		t.Fatalf("registered game = (%q, %v), want (%q, true)", gameID, exists, "game")
	}
}

func TestHandlerCreateGameReleasesReservationOnServiceError(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().CreatePrivateGame(gomock.Any(), gomock.Any()).Return(gameplay.CreateResult{}, gameplay.ErrInvalidTimeControl)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	raw := json.RawMessage(`{"type":"game.create","requestId":"request","payload":{"timeControl":"3+2","color":"black"}}`)

	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	message := receiveEnvelope(t, client)
	payload, ok := message.Payload.(protocol.ErrorPayload)
	if message.Type != protocol.ServerError || !ok || payload.Code != protocol.ErrorInvalidMessage {
		t.Fatalf("error envelope = %+v, want invalid-message error", message)
	}
	if err := registry.Reserve(client); err != nil {
		t.Fatalf("Reserve() after service error = %v, want released session", err)
	}
}

func TestHandlerJoinGameMapsRegistersAndInitializes(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().JoinPrivateGame(gomock.Any(), gameplay.NewJoinPrivateCommand("player", "game")).Return(
		gameplay.JoinResult{GameID: "game", Color: chess.Black},
		nil,
	)
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "peer",
		BlackProfileID: "player",
		Outcome:        chess.NoOutcome,
	}, nil)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	raw := json.RawMessage(`{"type":"game.join","requestId":"request","payload":{"gameId":"game"}}`)

	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	joined := receiveEnvelope(t, client)
	payload, ok := joined.Payload.(protocol.GameJoinedPayload)
	if joined.Type != protocol.ServerGameJoined || joined.RequestID != "request" || !ok || payload.GameID != "game" || payload.Color != protocol.ColorBlack {
		t.Fatalf("joined envelope = %+v, want black game.joined response", joined)
	}
	initial := receiveEnvelope(t, client)
	if initial.Type != protocol.ServerGameInitial {
		t.Fatalf("initial envelope = %+v, want game.initial", initial)
	}
	if gameID, exists := registry.GameID(client); !exists || gameID != "game" {
		t.Fatalf("registered game = (%q, %v), want (%q, true)", gameID, exists, "game")
	}
}

func TestHandlerMakeMoveBroadcastsAuthoritativeResult(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().MakeMove(gomock.Any(), gameplay.MoveCommand{
		GameID:          "game",
		ProfileID:       "player",
		Move:            "e4",
		Notation:        gameplay.MoveNotationSAN,
		ExpectedVersion: 1,
	}).Return(gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		Version:        2,
		WhiteProfileID: "player",
		BlackProfileID: "peer",
		WhiteRemaining: 9 * time.Minute,
		BlackRemaining: 8 * time.Minute,
		LastMoveSAN:    "e4",
		Outcome:        chess.NoOutcome,
	}, nil)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	peer := newQueuedSession("peer")
	if err := registry.Add("game", client); err != nil {
		t.Fatalf("Add(client) error = %v", err)
	}
	if err := registry.Add("game", peer); err != nil {
		t.Fatalf("Add(peer) error = %v", err)
	}
	raw := json.RawMessage(`{"type":"game.move","requestId":"request","payload":{"gameId":"game","move":"e4","notation":"san","expectedVersion":1}}`)

	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	for name, session := range map[string]*Session{"source": client, "peer": peer} {
		message := receiveEnvelope(t, session)
		payload, ok := message.Payload.(protocol.GameStatePayload)
		if message.Type != protocol.ServerGameState || !ok || payload.FEN != "fen" || payload.LastMove != "e4" || payload.Version != 2 {
			t.Fatalf("%s envelope = %+v, want authoritative game state", name, message)
		}
		if payload.White == nil || payload.Black == nil {
			t.Fatalf("%s player state = (%+v, %+v), want both players", name, payload.White, payload.Black)
		}
		if !payload.White.Connected || !payload.Black.Connected {
			t.Fatalf("%s connected state = (%v, %v), want both connected", name, payload.White.Connected, payload.Black.Connected)
		}
		if payload.White.Notation != protocol.MoveNotationUCI || payload.Black.Notation != protocol.MoveNotationUCI {
			t.Fatalf("%s notation = (%q, %q), want UCI", name, payload.White.Notation, payload.Black.Notation)
		}
		if payload.White.ProfileID != "player" || payload.Black.ProfileID != "peer" {
			t.Fatalf("%s profile IDs = (%q, %q), want (player, peer)", name, payload.White.ProfileID, payload.Black.ProfileID)
		}
		if payload.White.RemainingMilliseconds == nil || *payload.White.RemainingMilliseconds != (9*time.Minute).Milliseconds() {
			t.Fatalf("%s white remaining = %v, want 9 minutes", name, payload.White.RemainingMilliseconds)
		}
		if payload.Black.RemainingMilliseconds == nil || *payload.Black.RemainingMilliseconds != (8*time.Minute).Milliseconds() {
			t.Fatalf("%s black remaining = %v, want 8 minutes", name, payload.Black.RemainingMilliseconds)
		}
	}
}

func TestAwaitMatchRegistersAndInitializesSession(t *testing.T) {
	t.Parallel()

	state := gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "white",
		BlackProfileID: "black",
		Outcome:        chess.NoOutcome,
	}
	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(state, nil).Times(2)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	white := newQueuedSession("white")
	black := newQueuedSession("black")
	if err := registry.Queue(white); err != nil {
		t.Fatalf("Queue(white) error = %v", err)
	}
	if err := registry.Queue(black); err != nil {
		t.Fatalf("Queue(black) error = %v", err)
	}

	whiteResults := matchResults(gameplay.MatchResult{GameID: "game", Color: chess.White})
	blackResults := matchResults(gameplay.MatchResult{GameID: "game", Color: chess.Black})
	done := make(chan struct{}, 2)
	go func() {
		handler.awaitMatch(context.Background(), white, "white-request", gameplay.MatchTicket{Result: whiteResults})
		done <- struct{}{}
	}()
	go func() {
		handler.awaitMatch(context.Background(), black, "black-request", gameplay.MatchTicket{Result: blackResults})
		done <- struct{}{}
	}()
	<-done
	<-done

	for name, session := range map[string]*Session{"white": white, "black": black} {
		matched := receiveEnvelope(t, session)
		if matched.Type != protocol.ServerMatchFound {
			t.Fatalf("%s match envelope = %+v, want match.found", name, matched)
		}
		initial := receiveEnvelope(t, session)
		payload, ok := initial.Payload.(protocol.GameStatePayload)
		if initial.Type != protocol.ServerGameInitial || !ok {
			t.Fatalf("%s initial envelope = %+v, want game.initial", name, initial)
		}
		if payload.White == nil || payload.Black == nil || !payload.White.Connected || !payload.Black.Connected {
			t.Fatalf("%s connected players = (%+v, %+v), want both connected", name, payload.White, payload.Black)
		}
		if gameID, exists := registry.GameID(session); !exists || gameID != "game" {
			t.Fatalf("%s registered game = (%q, %v), want (%q, true)", name, gameID, exists, "game")
		}
	}
}

func TestAwaitMatchStopsWhenInitialStateFails(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{}, gameplay.ErrGameNotFound)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	peer := newQueuedSession("peer")
	if err := registry.Add("game", peer); err != nil {
		t.Fatalf("Add(peer) error = %v", err)
	}
	results := matchResults(gameplay.MatchResult{GameID: "game", Color: chess.White})

	handler.awaitMatch(context.Background(), client, "request", gameplay.MatchTicket{Result: results})

	matched := receiveEnvelope(t, client)
	if matched.Type != protocol.ServerMatchFound {
		t.Fatalf("first envelope type = %q, want %q", matched.Type, protocol.ServerMatchFound)
	}
	failed := receiveEnvelope(t, client)
	payload, ok := failed.Payload.(protocol.ErrorPayload)
	if failed.Type != protocol.ServerError || !ok || payload.Code != protocol.ErrorGameNotFound {
		t.Fatalf("second envelope = %+v, want game-not-found error", failed)
	}
	if len(client.outgoing) != 0 {
		t.Fatalf("received %d envelopes after state error, want no game.initial", len(client.outgoing))
	}
}

func TestHandlerRejectsPrivateGameWhileQueued(t *testing.T) {
	t.Parallel()

	service := NewMockGameService(gomock.NewController(t))
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	if err := registry.Queue(client); err != nil {
		t.Fatalf("Queue() error = %v", err)
	}

	raw := json.RawMessage(`{"type":"game.create","requestId":"request","payload":{"timeControl":"3+2","color":"white"}}`)
	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	message := receiveEnvelope(t, client)
	payload, ok := message.Payload.(protocol.ErrorPayload)
	if message.Type != protocol.ServerError || !ok || payload.Message != "session is already in a game or matchmaking" {
		t.Fatalf("error envelope = %+v, want queued-session rejection", message)
	}
}

func matchResults(result gameplay.MatchResult) <-chan gameplay.MatchResult {
	results := make(chan gameplay.MatchResult, 1)
	results <- result
	close(results)
	return results
}
