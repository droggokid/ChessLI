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
		{name: "not implemented", raw: json.RawMessage(`{"type":"game.resign","requestId":"request"}`), wantRequestID: "request", wantCode: protocol.ErrorNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newQueuedSession("player")
			service := gameplay.NewMockService(gomock.NewController(t))
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

	service := gameplay.NewMockService(gomock.NewController(t))
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

func TestHandlerMakeMoveBroadcastsAuthoritativeResult(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockService(gomock.NewController(t))
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

	state := gameplay.GameSnapshot{GameID: "game", FEN: "fen", Outcome: chess.NoOutcome}
	service := gameplay.NewMockService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(state, nil)
	registry := NewGameSessions()
	handler := NewHandler(service, registry)
	client := newQueuedSession("player")
	results := make(chan gameplay.MatchResult, 1)
	results <- gameplay.MatchResult{GameID: "game", Color: chess.White}
	close(results)

	handler.awaitMatch(context.Background(), client, "request", gameplay.MatchTicket{Result: results})

	matched := receiveEnvelope(t, client)
	if matched.Type != protocol.ServerMatchFound || matched.RequestID != "request" {
		t.Fatalf("match envelope = %+v, want match.found with request ID", matched)
	}
	initial := receiveEnvelope(t, client)
	if initial.Type != protocol.ServerGameInitial {
		t.Fatalf("initial envelope type = %q, want %q", initial.Type, protocol.ServerGameInitial)
	}
	if gameID, exists := registry.GameID(client); !exists || gameID != "game" {
		t.Fatalf("registered game = (%q, %v), want (%q, true)", gameID, exists, "game")
	}
}

func TestAwaitMatchStopsWhenInitialStateFails(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{}, gameplay.ErrGameNotFound)
	handler := NewHandler(service, NewGameSessions())
	client := newQueuedSession("player")
	results := make(chan gameplay.MatchResult, 1)
	results <- gameplay.MatchResult{GameID: "game", Color: chess.White}
	close(results)

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
