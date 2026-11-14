package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ChessLI/internal/game"
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
		{name: "unknown envelope field is ignored", raw: json.RawMessage(`{"type":"unknown","requestId":"request","future":true}`), wantRequestID: "request", wantCode: protocol.ErrorUnknownType},
		{name: "missing draw game ID", raw: json.RawMessage(`{"type":"draw.offer","requestId":"request","payload":{}}`), wantRequestID: "request", wantCode: protocol.ErrorInvalidMessage},
		{name: "missing draw offer ID", raw: json.RawMessage(`{"type":"draw.accept","requestId":"request","payload":{"gameId":"game"}}`), wantRequestID: "request", wantCode: protocol.ErrorInvalidMessage},
		{name: "missing resign game ID", raw: json.RawMessage(`{"type":"game.resign","requestId":"request","payload":{}}`), wantRequestID: "request", wantCode: protocol.ErrorInvalidMessage},
		{name: "missing resume game ID", raw: json.RawMessage(`{"type":"game.resume","requestId":"request","payload":{}}`), wantRequestID: "request", wantCode: protocol.ErrorInvalidMessage},
		{name: "missing move game ID", raw: json.RawMessage(`{"type":"game.move","requestId":"request","payload":{"move":"e2e4","notation":"uci","expectedVersion":0}}`), wantRequestID: "request", wantCode: protocol.ErrorInvalidMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newQueuedSession("player")
			service := gameplay.NewMockGameService(gomock.NewController(t))
			handler := newHandler(service, newGameSessions())
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

func TestHandlerGameActionsBroadcastState(t *testing.T) {
	tests := []struct {
		name   string
		raw    json.RawMessage
		expect func(*gameplay.MockGameService) gameplay.GameSnapshot
	}{
		{
			name: "resign",
			raw:  json.RawMessage(`{"type":"game.resign","requestId":"request","payload":{"gameId":"game"}}`),
			expect: func(service *gameplay.MockGameService) gameplay.GameSnapshot {
				state := gameplay.GameSnapshot{GameID: "game", Outcome: game.OutcomeBlackWin, Termination: game.TerminationResignation}
				service.EXPECT().Resign(gomock.Any(), gameplay.ResignCommand{GameID: "game", ProfileID: "white"}).Return(state, nil)
				return state
			},
		},
		{
			name: "offer draw",
			raw:  json.RawMessage(`{"type":"draw.offer","requestId":"request","payload":{"gameId":"game"}}`),
			expect: func(service *gameplay.MockGameService) gameplay.GameSnapshot {
				state := gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", PendingDrawOffer: &gameplay.DrawOffer{OfferID: "offer", OfferedBy: "white"}}
				service.EXPECT().OfferDraw(gomock.Any(), gameplay.OfferDrawCommand{GameID: "game", ProfileID: "white"}).Return(state, nil)
				return state
			},
		},
		{
			name: "accept draw",
			raw:  json.RawMessage(`{"type":"draw.accept","requestId":"request","payload":{"gameId":"game","offerId":"offer"}}`),
			expect: func(service *gameplay.MockGameService) gameplay.GameSnapshot {
				state := gameplay.GameSnapshot{GameID: "game", Outcome: game.OutcomeDraw, Termination: game.TerminationDrawAgreement, Version: 1}
				service.EXPECT().AcceptDraw(gomock.Any(), gameplay.DrawOfferResponseCommand{GameID: "game", ProfileID: "white", OfferID: "offer"}).Return(state, nil)
				return state
			},
		},
		{
			name: "decline draw",
			raw:  json.RawMessage(`{"type":"draw.decline","requestId":"request","payload":{"gameId":"game","offerId":"offer"}}`),
			expect: func(service *gameplay.MockGameService) gameplay.GameSnapshot {
				state := gameplay.GameSnapshot{GameID: "game", Version: 2}
				service.EXPECT().DeclineDraw(gomock.Any(), gameplay.DrawOfferResponseCommand{GameID: "game", ProfileID: "white", OfferID: "offer"}).Return(state, nil)
				return state
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := gameplay.NewMockGameService(gomock.NewController(t))
			want := tt.expect(service)
			registry := newGameSessions()
			client := newQueuedSession("white")
			peer := newQueuedSession("black")
			if err := registry.Add("game", client); err != nil {
				t.Fatalf("Add(client) error = %v", err)
			}
			if err := registry.Add("game", peer); err != nil {
				t.Fatalf("Add(peer) error = %v", err)
			}

			if err := newHandler(service, registry).Handle(context.Background(), client, tt.raw); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			for name, session := range map[string]*Session{"source": client, "peer": peer} {
				message := receiveEnvelope(t, session)
				payload, ok := message.Payload.(protocol.GameStatePayload)
				if message.Type != protocol.ServerGameState || !ok || payload.Version != want.Version {
					t.Fatalf("%s envelope = %+v, want game state version %d", name, message, want.Version)
				}
				if name == "source" && message.RequestID != "request" {
					t.Fatalf("source request ID = %q, want request", message.RequestID)
				}
				if name == "peer" && message.RequestID != "" {
					t.Fatalf("peer request ID = %q, want empty", message.RequestID)
				}
			}
		})
	}
}

func TestHandlerCreateGameMapsAndRegisters(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().CreatePrivateGame(gomock.Any(), gameplay.CreatePrivateCommand{
		ProfileID:       "player",
		Initial:         3 * time.Minute,
		Increment:       2 * time.Second,
		ColorPreference: gameplay.ColorBlack,
	}).Return(gameplay.GameAssignment{GameID: "game", Color: chess.Black}, nil)
	registry := newGameSessions()
	handler := newHandler(service, registry)
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
	if !registry.IsConnected("game", "player") {
		t.Fatal("client was not registered to game")
	}
}

func TestHandlerCreateGameReleasesReservationOnServiceError(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().CreatePrivateGame(gomock.Any(), gomock.Any()).Return(gameplay.GameAssignment{}, gameplay.ErrInvalidTimeControl)
	registry := newGameSessions()
	handler := newHandler(service, registry)
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
	if err := registry.Hold(client); err != nil {
		t.Fatalf("Hold() after service error = %v, want released session", err)
	}
}

func TestHandlerJoinGameMapsRegistersAndInitializes(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().JoinPrivateGame(gomock.Any(), gameplay.JoinPrivateCommand{ProfileID: "player", GameID: "game"}).Return(
		gameplay.GameAssignment{GameID: "game", Color: chess.Black},
		nil,
	)
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "peer",
		BlackProfileID: "player",
		Outcome:        game.OutcomeNone,
	}, nil)
	registry := newGameSessions()
	handler := newHandler(service, registry)
	client := newQueuedSession("player")
	raw := json.RawMessage(`{"type":"game.join","requestId":"request","payload":{"gameId":"game"}}`)

	if err := handler.Handle(context.Background(), client, raw); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	joined := receiveEnvelope(t, client)
	payload, ok := joined.Payload.(protocol.GameCreatedPayload)
	if joined.Type != protocol.ServerGameJoined || joined.RequestID != "request" || !ok || payload.GameID != "game" || payload.Color != protocol.ColorBlack {
		t.Fatalf("joined envelope = %+v, want black game.joined response", joined)
	}
	initial := receiveEnvelope(t, client)
	if initial.Type != protocol.ServerGameInitial {
		t.Fatalf("initial envelope = %+v, want game.initial", initial)
	}
	if !registry.IsConnected("game", "player") {
		t.Fatal("client was not registered to game")
	}
}

func TestHandlerResumeGameRegistersAndInitializes(t *testing.T) {
	t.Parallel()

	state := gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "player",
		BlackProfileID: "peer",
		Outcome:        game.OutcomeNone,
	}
	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().ResumeGame(gomock.Any(), gameplay.ResumeGameCommand{
		GameID:    "game",
		ProfileID: "player",
	}).Return(state, nil)
	registry := newGameSessions()
	client := newQueuedSession("player")

	if err := newHandler(service, registry).Handle(context.Background(), client, json.RawMessage(`{"type":"game.resume","requestId":"request","payload":{"gameId":"game"}}`)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	resumed := receiveEnvelope(t, client)
	payload, ok := resumed.Payload.(protocol.GameCreatedPayload)
	if resumed.Type != protocol.ServerGameResumed || resumed.RequestID != "request" || !ok || payload.GameID != "game" || payload.Color != protocol.ColorWhite {
		t.Fatalf("resumed envelope = %+v, want white game.resumed response", resumed)
	}
	initial := receiveEnvelope(t, client)
	if initial.Type != protocol.ServerGameInitial {
		t.Fatalf("initial envelope = %+v, want game.initial", initial)
	}
	if !registry.IsConnected("game", "player") {
		t.Fatal("client was not registered to game")
	}
}

func TestHandlerResumeFinishedGameDoesNotRegisterSession(t *testing.T) {
	t.Parallel()

	state := gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "player",
		BlackProfileID: "peer",
		Outcome:        game.OutcomeWhiteWin,
	}
	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().ResumeGame(gomock.Any(), gameplay.ResumeGameCommand{
		GameID:    "game",
		ProfileID: "player",
	}).Return(state, nil)
	registry := newGameSessions()
	client := newQueuedSession("player")

	if err := newHandler(service, registry).Handle(context.Background(), client, json.RawMessage(`{"type":"game.resume","requestId":"request","payload":{"gameId":"game"}}`)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if message := receiveEnvelope(t, client); message.Type != protocol.ServerGameResumed {
		t.Fatalf("first envelope = %+v, want game.resumed", message)
	}
	initial := receiveEnvelope(t, client)
	if initial.Type != protocol.ServerGameInitial {
		t.Fatalf("second envelope = %+v, want game.initial", initial)
	}
	payload, ok := initial.Payload.(protocol.GameStatePayload)
	if !ok || payload.White == nil || payload.Black == nil || payload.White.Connected || payload.Black.Connected {
		t.Fatalf("finished initial state = %+v, want both players disconnected", initial)
	}
	if registry.IsConnected("game", "player") {
		t.Fatal("finished game registered the session")
	}
	if err := registry.Hold(client); err != nil {
		t.Fatalf("Hold() after finished resume error = %v", err)
	}
}

func TestHandlerMakeMoveBroadcastsAuthoritativeResult(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockGameService(gomock.NewController(t))
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
		Outcome:        game.OutcomeNone,
	}, nil)
	registry := newGameSessions()
	handler := newHandler(service, registry)
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

func TestMatchAdmissionRegistersAndInitializesSession(t *testing.T) {
	t.Parallel()

	state := gameplay.GameSnapshot{
		GameID:         "game",
		FEN:            "fen",
		WhiteProfileID: "white",
		BlackProfileID: "black",
		Outcome:        game.OutcomeNone,
	}
	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(state, nil).Times(2)
	registry := newGameSessions()
	handler := newHandler(service, registry)
	white := newQueuedSession("white")
	black := newQueuedSession("black")
	if err := registry.Hold(white); err != nil {
		t.Fatalf("Hold(white) error = %v", err)
	}
	if err := registry.Hold(black); err != nil {
		t.Fatalf("Hold(black) error = %v", err)
	}

	if err := handler.admitMatch(t.Context(), white, "white-request", "game"); err != nil {
		t.Fatal(err)
	}
	if err := handler.admitMatch(t.Context(), black, "black-request", "game"); err != nil {
		t.Fatal(err)
	}

	for name, session := range map[string]*Session{"white": white, "black": black} {
		matched := receiveEnvelope(t, session)
		if matched.Type != protocol.ServerMatchFound {
			t.Fatalf("%s match envelope = %+v, want match.found", name, matched)
		}
		assignment, ok := matched.Payload.(protocol.GameCreatedPayload)
		if !ok || assignment.Color != protocol.Color(name) {
			t.Fatalf("%s assignment = %+v, want matching player color", name, matched.Payload)
		}
		initial := receiveEnvelope(t, session)
		payload, ok := initial.Payload.(protocol.GameStatePayload)
		if initial.Type != protocol.ServerGameInitial || !ok {
			t.Fatalf("%s initial envelope = %+v, want game.initial", name, initial)
		}
		if payload.White == nil || payload.Black == nil {
			t.Fatalf("%s player state = (%+v, %+v), want both players", name, payload.White, payload.Black)
		}
		if session.profileID == "white" && !payload.White.Connected {
			t.Fatalf("%s white player was not connected", name)
		}
		if session.profileID == "black" && !payload.Black.Connected {
			t.Fatalf("%s black player was not connected", name)
		}
		if !registry.IsConnected("game", session.profileID) {
			t.Fatalf("%s was not registered to game", name)
		}
		// The earlier admission receives a presence update after its initial state.
		if len(session.outgoing) != 0 {
			update := receiveEnvelope(t, session)
			if update.Type != protocol.ServerGameState || update.RequestID != "" {
				t.Fatalf("presence update = %+v", update)
			}
			payload = update.Payload.(protocol.GameStatePayload)
		}
		if !payload.White.Connected || !payload.Black.Connected {
			t.Fatalf("%s never learned both players were connected: %+v", name, payload)
		}
	}
}

func TestMatchAdmissionStopsWhenInitialStateFails(t *testing.T) {
	t.Parallel()

	service := gameplay.NewMockGameService(gomock.NewController(t))
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{}, gameplay.ErrGameNotFound)
	registry := newGameSessions()
	handler := newHandler(service, registry)
	client := newQueuedSession("player")
	peer := newQueuedSession("peer")
	if err := registry.Add("game", peer); err != nil {
		t.Fatalf("Add(peer) error = %v", err)
	}
	handler.admitMatch(t.Context(), client, "request", "game")

	failed := receiveEnvelope(t, client)
	payload, ok := failed.Payload.(protocol.ErrorPayload)
	if failed.Type != protocol.ServerError || !ok || payload.Code != protocol.ErrorGameNotFound {
		t.Fatalf("second envelope = %+v, want game-not-found error", failed)
	}
	if len(client.outgoing) != 0 {
		t.Fatalf("received %d envelopes after state error, want no game.initial", len(client.outgoing))
	}
	if err := registry.Hold(client); err != nil {
		t.Fatalf("failed matchmaking admission occupied the session: %v", err)
	}
}

func TestHandlerRejectsGameAdmissionWhileBusy(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"queued", "seated"} {
		for _, action := range []struct {
			name string
			raw  json.RawMessage
		}{
			{"create", json.RawMessage(`{"type":"game.create","requestId":"request","payload":{"timeControl":"3+2","color":"white"}}`)},
			{"join", json.RawMessage(`{"type":"game.join","requestId":"request","payload":{"gameId":"game"}}`)},
			{"resume", json.RawMessage(`{"type":"game.resume","requestId":"request","payload":{"gameId":"game"}}`)},
		} {
			t.Run(status+"/"+action.name, func(t *testing.T) {
				service := gameplay.NewMockGameService(gomock.NewController(t))
				registry := newGameSessions()
				client := newQueuedSession("player")
				var err error
				if status == "queued" {
					err = registry.Hold(client)
				} else {
					err = registry.Add("existing-game", client)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := newHandler(service, registry).Handle(context.Background(), client, action.raw); err != nil {
					t.Fatalf("Handle() error = %v", err)
				}
				message := receiveEnvelope(t, client)
				payload, ok := message.Payload.(protocol.ErrorPayload)
				if message.Type != protocol.ServerError || message.RequestID != "request" || !ok || payload.Message != sessionUnavailableMessage {
					t.Fatalf("error envelope = %+v, want busy-session rejection", message)
				}
			})
		}
	}
}

func TestHandlerMatchmakingPublishesBothEntriesBeforeReturning(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	first, second := new(gameplay.MatchmakingEntry), new(gameplay.MatchmakingEntry)
	service.EXPECT().EnterMatchmaking(gomock.Any(), gameplay.EnterMatchmakingCommand{ProfileID: "white", Initial: time.Minute}).Return(gameplay.MatchmakingResult{Entry: first}, nil)
	service.EXPECT().EnterMatchmaking(gomock.Any(), gameplay.EnterMatchmakingCommand{ProfileID: "black", Initial: time.Minute}).Return(gameplay.MatchmakingResult{Entry: second, Opponent: first, GameID: "game"}, nil)
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", BlackProfileID: "black"}, nil).Times(2)
	handler := newHandler(service, newGameSessions())
	white, black := newQueuedSession("white"), newQueuedSession("black")
	for _, client := range []*Session{white, black} {
		raw := json.RawMessage(`{"type":"matchmaking.enter","requestId":"` + string(client.profileID) + `","payload":{"timeControl":"1+0"}}`)
		if err := handler.Handle(t.Context(), client, raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, client := range []*Session{white, black} {
		for _, want := range []protocol.ServerMessageType{protocol.ServerMatchmakingEntered, protocol.ServerMatchFound, protocol.ServerGameInitial} {
			got := receiveEnvelope(t, client)
			if got.Type != want {
				t.Fatalf("%s response = %s, want %s", client.profileID, got.Type, want)
			}
			if want != protocol.ServerGameInitial && got.RequestID != string(client.profileID) {
				t.Fatalf("assignment lost entry request ID: %+v", got)
			}
		}
	}
	if len(handler.pending) != 0 {
		t.Fatal("matched entries remained pending")
	}
}

func TestHandlerMatchmakingCancelsUnacknowledgedEntry(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	entry := new(gameplay.MatchmakingEntry)
	service.EXPECT().EnterMatchmaking(gomock.Any(), gomock.Any()).Return(gameplay.MatchmakingResult{Entry: entry}, nil)
	service.EXPECT().CancelMatchmaking(entry)
	registry := newGameSessions()
	handler := newHandler(service, registry)
	client := newQueuedSession("player")
	for len(client.outgoing) < cap(client.outgoing) {
		client.outgoing <- protocol.ServerEnvelope{}
	}
	err := handler.Handle(t.Context(), client, json.RawMessage(`{"type":"matchmaking.enter","payload":{"timeControl":"1+0"}}`))
	if !errors.Is(err, protocol.ErrSessionQueueFull) {
		t.Fatalf("admission error = %v, want queue full", err)
	}
	if len(handler.pending) != 0 || registry.IsBusy(client) {
		t.Fatal("unacknowledged entry retained a session reservation")
	}
}

func TestHandlerMatchmakingPublishesPeerAfterSourceCancellation(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	first, second := new(gameplay.MatchmakingEntry), new(gameplay.MatchmakingEntry)
	service.EXPECT().EnterMatchmaking(gomock.Any(), gameplay.EnterMatchmakingCommand{ProfileID: "white", Initial: time.Minute}).Return(gameplay.MatchmakingResult{Entry: first}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	service.EXPECT().EnterMatchmaking(gomock.Any(), gameplay.EnterMatchmakingCommand{ProfileID: "black", Initial: time.Minute}).DoAndReturn(
		func(context.Context, gameplay.EnterMatchmakingCommand) (gameplay.MatchmakingResult, error) {
			cancel()
			return gameplay.MatchmakingResult{Entry: second, Opponent: first, GameID: "game"}, nil
		})
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", BlackProfileID: "black"}, nil)
	handler := newHandler(service, newGameSessions())
	white, black := newQueuedSession("white"), newQueuedSession("black")
	raw := json.RawMessage(`{"type":"matchmaking.enter","requestId":"match","payload":{"timeControl":"1+0"}}`)
	if err := handler.Handle(t.Context(), white, raw); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(ctx, black, raw); !errors.Is(err, context.Canceled) {
		t.Fatalf("admission error = %v, want canceled", err)
	}
	for _, want := range []protocol.ServerMessageType{protocol.ServerMatchmakingEntered, protocol.ServerMatchFound, protocol.ServerGameInitial} {
		if got := receiveEnvelope(t, white); got.Type != want {
			t.Fatalf("surviving peer response = %s, want %s", got.Type, want)
		}
	}
	if len(handler.pending) != 0 || handler.gameSessions.IsBusy(black) {
		t.Fatal("canceled source retained a reservation")
	}
}

func TestHandlerTerminalStateReachesPeerWhenSourceStops(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	registry := newGameSessions()
	source, peer := newQueuedSession("white"), newQueuedSession("black")
	for _, session := range []*Session{source, peer} {
		if err := registry.Add("game", session); err != nil {
			t.Fatal(err)
		}
	}
	service.EXPECT().Resign(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, gameplay.ResignCommand) (gameplay.GameSnapshot, error) {
			source.running.Store(false)
			return gameplay.GameSnapshot{GameID: "game", Outcome: game.OutcomeBlackWin, Termination: game.TerminationResignation}, nil
		})
	err := newHandler(service, registry).Handle(context.Background(), source,
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

func TestJoinPublishesInitialStateAfterRequestCancellation(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.EXPECT().JoinPrivateGame(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, gameplay.JoinPrivateCommand) (gameplay.GameAssignment, error) {
			cancel()
			return gameplay.GameAssignment{GameID: "game", Color: chess.Black}, nil
		})
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).DoAndReturn(
		func(ctx context.Context, _ identity.GameID) (gameplay.GameSnapshot, error) {
			if err := ctx.Err(); err != nil {
				t.Fatalf("committed join snapshot context: %v", err)
			}
			return gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", BlackProfileID: "black"}, nil
		})
	registry := newGameSessions()
	client, peer := newQueuedSession("black"), newQueuedSession("white")
	if err := registry.Add("game", peer); err != nil {
		t.Fatal(err)
	}
	err := newHandler(service, registry).Handle(ctx, client, json.RawMessage(`{"type":"game.join","payload":{"gameId":"game"}}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("join error = %v, want canceled", err)
	}
	message := receiveEnvelope(t, peer)
	state := message.Payload.(protocol.GameStatePayload)
	if message.Type != protocol.ServerGameInitial || message.RequestID != "" || state.Black == nil || !state.Black.Connected {
		t.Fatalf("peer did not receive committed initial state: %+v", message)
	}
}

func TestResumePublishesPresenceAfterRequestCancellation(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.EXPECT().ResumeGame(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, gameplay.ResumeGameCommand) (gameplay.GameSnapshot, error) {
		cancel()
		return gameplay.GameSnapshot{GameID: "game", WhiteProfileID: "white", BlackProfileID: "black"}, nil
	})
	registry := newGameSessions()
	client, peer := newQueuedSession("black"), newQueuedSession("white")
	if err := registry.Add("game", peer); err != nil {
		t.Fatal(err)
	}
	err := newHandler(service, registry).Handle(ctx, client, json.RawMessage(`{"type":"game.resume","payload":{"gameId":"game"}}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resume error = %v, want canceled", err)
	}
	message := receiveEnvelope(t, peer)
	if !message.Payload.(protocol.GameStatePayload).Black.Connected {
		t.Fatal("peer did not receive committed presence")
	}
}

func TestMatchAdmissionFinishedGameIsReadOnly(t *testing.T) {
	service := gameplay.NewMockGameService(gomock.NewController(t))
	state := gameplay.GameSnapshot{
		GameID: "game", WhiteProfileID: "white", BlackProfileID: "black",
		Outcome: game.OutcomeBlackWin, Termination: game.TerminationResignation,
	}
	service.EXPECT().GameState(gomock.Any(), identity.GameID("game")).Return(state, nil)
	registry := newGameSessions()
	client, peer := newQueuedSession("white"), newQueuedSession("black")
	if err := registry.Hold(client); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add("game", peer); err != nil {
		t.Fatal(err)
	}
	newHandler(service, registry).admitMatch(t.Context(), client, "match", "game")
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
			if err := server.messageHandler.gameSessions.Add("game", peer); err != nil {
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
				if err := server.messageHandler.gameSessions.Add("game", client); err != nil {
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
				if err := server.messageHandler.gameSessions.Hold(session); err != nil {
					t.Fatalf("session remained in finished game: %v", err)
				}
			}
		})
	}
}
