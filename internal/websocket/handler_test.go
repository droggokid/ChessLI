package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
	}).Return(gameplay.MoveResult{GameID: "game", FEN: "fen", SAN: "e4", Version: 2, Outcome: chess.NoOutcome}, nil)
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
	}
}

func TestMapTimeControlPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset        protocol.TimeControlPreset
		wantInitial   time.Duration
		wantIncrement time.Duration
		wantErr       bool
	}{
		{preset: protocol.TimeControlBullet1Plus0, wantInitial: time.Minute},
		{preset: protocol.TimeControlBullet1Plus1, wantInitial: time.Minute, wantIncrement: time.Second},
		{preset: protocol.TimeControlBullet2Plus1, wantInitial: 2 * time.Minute, wantIncrement: time.Second},
		{preset: protocol.TimeControlBlitz3Plus0, wantInitial: 3 * time.Minute},
		{preset: protocol.TimeControlBlitz3Plus2, wantInitial: 3 * time.Minute, wantIncrement: 2 * time.Second},
		{preset: protocol.TimeControlBlitz5Plus0, wantInitial: 5 * time.Minute},
		{preset: protocol.TimeControlRapid10Plus0, wantInitial: 10 * time.Minute},
		{preset: protocol.TimeControlRapid10Plus5, wantInitial: 10 * time.Minute, wantIncrement: 5 * time.Second},
		{preset: protocol.TimeControlRapid15Plus10, wantInitial: 15 * time.Minute, wantIncrement: 10 * time.Second},
		{preset: protocol.TimeControlClassical30Plus0, wantInitial: 30 * time.Minute},
		{preset: protocol.TimeControlPreset("custom"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			t.Parallel()

			initial, increment, err := mapTimeControlPreset(tt.preset)
			if (err != nil) != tt.wantErr {
				t.Fatalf("mapTimeControlPreset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if initial != tt.wantInitial || increment != tt.wantIncrement {
				t.Fatalf("mapTimeControlPreset() = (%v, %v), want (%v, %v)", initial, increment, tt.wantInitial, tt.wantIncrement)
			}
		})
	}
}

func TestProtocolToGameplayMappings(t *testing.T) {
	t.Parallel()

	t.Run("move notation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input   protocol.MoveNotation
			want    gameplay.MoveNotation
			wantErr bool
		}{
			{input: "", want: gameplay.MoveNotationUCI},
			{input: protocol.MoveNotationUCI, want: gameplay.MoveNotationUCI},
			{input: protocol.MoveNotationSAN, want: gameplay.MoveNotationSAN},
			{input: protocol.MoveNotationLAN, want: gameplay.MoveNotationLAN},
			{input: "pgn", wantErr: true},
		}

		for _, tt := range tests {
			got, err := mapMoveNotation(tt.input)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("mapMoveNotation(%q) = (%v, %v), want (%v, wantErr=%v)", tt.input, got, err, tt.want, tt.wantErr)
			}
		}
	})

	t.Run("color preference", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input   protocol.ColorPreference
			want    gameplay.ColorPreference
			wantErr bool
		}{
			{input: "", want: gameplay.ColorRandom},
			{input: protocol.ColorPreferenceRandom, want: gameplay.ColorRandom},
			{input: protocol.ColorPreferenceWhite, want: gameplay.ColorWhite},
			{input: protocol.ColorPreferenceBlack, want: gameplay.ColorBlack},
			{input: "green", wantErr: true},
		}

		for _, tt := range tests {
			got, err := mapColorPreference(tt.input)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("mapColorPreference(%q) = (%v, %v), want (%v, wantErr=%v)", tt.input, got, err, tt.want, tt.wantErr)
			}
		}
	})
}

func TestMapApplicationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err         error
		wantCode    protocol.ErrorCode
		wantMessage string
	}{
		{gameplay.ErrGameNotFound, protocol.ErrorGameNotFound, "game not found"},
		{gameplay.ErrGameFull, protocol.ErrorGameFull, "game is full"},
		{gameplay.ErrNotParticipant, protocol.ErrorNotPlayer, "not a player in this game"},
		{gameplay.ErrNotYourTurn, protocol.ErrorNotYourTurn, "not your turn"},
		{gameplay.ErrIllegalMove, protocol.ErrorIllegalMove, "illegal move"},
		{gameplay.ErrGameFinished, protocol.ErrorGameFinished, "game is finished"},
		{gameplay.ErrGameNotReady, protocol.ErrorIllegalMove, "game is waiting for another player"},
		{gameplay.ErrInvalidColorPreference, protocol.ErrorInvalidMessage, "invalid color preference"},
		{gameplay.ErrInvalidTimeControl, protocol.ErrorInvalidMessage, "invalid time control"},
		{gameplay.ErrUnsupportedNotation, protocol.ErrorInvalidMessage, "unsupported move notation"},
		{gameplay.ErrAlreadyParticipant, protocol.ErrorInvalidMessage, "already a participant in this game"},
		{gameplay.ErrAlreadyQueued, protocol.ErrorInvalidMessage, "already queued for matchmaking"},
		{gameplay.ErrNoCompatibleOpponent, protocol.ErrorInvalidMessage, "no compatible opponent available"},
		{gameplay.ErrStaleGameVersion, protocol.ErrorStaleGameVersion, "game state is stale"},
		{errors.New("unexpected"), protocol.ErrorInternal, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.wantCode)+"_"+tt.wantMessage, func(t *testing.T) {
			t.Parallel()

			code, message := mapApplicationError(tt.err)
			if code != tt.wantCode || message != tt.wantMessage {
				t.Fatalf("mapApplicationError() = (%q, %q), want (%q, %q)", code, message, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

func TestMapOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outcome chess.Outcome
		method  chess.Method
		want    *protocol.GameOutcome
	}{
		{name: "in progress", outcome: chess.NoOutcome},
		{name: "unknown", outcome: chess.UnknownOutcome},
		{name: "white checkmate", outcome: chess.WhiteWon, method: chess.Checkmate, want: &protocol.GameOutcome{Result: protocol.ResultWhiteWin, Reason: protocol.GameOverCheckmate}},
		{name: "black resignation", outcome: chess.BlackWon, method: chess.Resignation, want: &protocol.GameOutcome{Result: protocol.ResultBlackWin, Reason: protocol.GameOverResignation}},
		{name: "draw agreement", outcome: chess.Draw, method: chess.DrawOffer, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverAgreement}},
		{name: "stalemate", outcome: chess.Draw, method: chess.Stalemate, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverStalemate}},
		{name: "fivefold repetition", outcome: chess.Draw, method: chess.FivefoldRepetition, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverThreefoldRepetition}},
		{name: "seventy-five move", outcome: chess.Draw, method: chess.SeventyFiveMoveRule, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverFiftyMoveRule}},
		{name: "insufficient material", outcome: chess.Draw, method: chess.InsufficientMaterial, want: &protocol.GameOutcome{Result: protocol.ResultDraw, Reason: protocol.GameOverInsufficientMaterial}},
		{name: "missing method", outcome: chess.Draw, method: chess.NoMethod},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := mapOutcome(tt.outcome, tt.method); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mapOutcome() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGameStatePayload(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil, NewGameSessions())
	state := gameplay.GameSnapshot{
		GameID:      "game",
		FEN:         "fen",
		Version:     3,
		LastMoveSAN: "Qh4#",
		Outcome:     chess.BlackWon,
		Method:      chess.Checkmate,
	}

	got := handler.gameStatePayload(state)
	if got.GameID != state.GameID || got.FEN != state.FEN || got.Version != state.Version || got.LastMove != state.LastMoveSAN {
		t.Fatalf("gameStatePayload() = %+v, want snapshot values %+v", got, state)
	}
	if got.Status != protocol.GameStatusFinished {
		t.Fatalf("gameStatePayload() status = %q, want %q", got.Status, protocol.GameStatusFinished)
	}
	wantOutcome := &protocol.GameOutcome{Result: protocol.ResultBlackWin, Reason: protocol.GameOverCheckmate}
	if !reflect.DeepEqual(got.Outcome, wantOutcome) {
		t.Fatalf("gameStatePayload() outcome = %+v, want %+v", got.Outcome, wantOutcome)
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
