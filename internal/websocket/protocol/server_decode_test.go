package protocol

import (
	"reflect"
	"testing"
)

func TestDecodeEventUsesProtocolPayloads(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    ServerEvent
	}{
		{
			"matchmaking admission",
			`{"type":"matchmaking.entered","requestId":"match-1","payload":{"timeControl":"10+0","futureField":true}}`,
			ServerEvent{Type: ServerMatchmakingEntered, RequestID: "match-1", Matchmaking: &EnterMatchmakingPayload{TimeControl: TimeControlRapid10Plus0}},
		},
		{
			"matchmaking assignment",
			`{"type":"matchmaking.found","requestId":"match-1","payload":{"gameId":"game-1","color":"black"}}`,
			ServerEvent{Type: ServerMatchFound, RequestID: "match-1", Assignment: &GameCreatedPayload{GameID: "game-1", Color: ColorBlack}},
		},
		{
			"ready",
			`{"type":"connection.ready","payload":{"profileId":"profile-1","futureField":true}}`,
			ServerEvent{Type: ServerConnectionReady, Ready: &ConnectionReadyPayload{ProfileID: "profile-1"}},
		},
		{
			"created",
			`{"type":"game.created","requestId":"create-1","payload":{"gameId":"game-1","color":"white"}}`,
			ServerEvent{Type: ServerGameCreated, RequestID: "create-1", Assignment: &GameCreatedPayload{GameID: "game-1", Color: ColorWhite}},
		},
		{
			"joined",
			`{"type":"game.joined","payload":{"gameId":"game-1","color":"black"}}`,
			ServerEvent{Type: ServerGameJoined, Assignment: &GameCreatedPayload{GameID: "game-1", Color: ColorBlack}},
		},
		{
			"resumed",
			`{"type":"game.resumed","payload":{"gameId":"game-1","color":"white"}}`,
			ServerEvent{Type: ServerGameResumed, Assignment: &GameCreatedPayload{GameID: "game-1", Color: ColorWhite}},
		},
		{
			"initial with version zero and absent optional fields",
			`{"type":"game.initial","payload":{"gameId":"game-1","fen":"position","status":"active","version":0}}`,
			ServerEvent{Type: ServerGameInitial, State: &GameStatePayload{GameID: "game-1", FEN: "position", Status: GameStatusActive}},
		},
		{
			"version above floating point integer precision",
			`{"type":"game.state","payload":{"gameId":"game-1","fen":"position","status":"active","version":9007199254740993}}`,
			ServerEvent{Type: ServerGameState, State: &GameStatePayload{GameID: "game-1", FEN: "position", Status: GameStatusActive, Version: 9007199254740993}},
		},
		{
			"state with player and outcome",
			`{"type":"game.state","requestId":"resign-1","payload":{"gameId":"game-1","fen":"position","status":"finished","version":2,"lastMove":"e4","white":{"profileId":"profile-1","color":"white","connected":false,"notation":"uci"},"outcome":{"result":"black_win","reason":"resignation"}}}`,
			ServerEvent{Type: ServerGameState, RequestID: "resign-1", State: &GameStatePayload{
				GameID: "game-1", FEN: "position", Status: GameStatusFinished, Version: 2, LastMove: "e4",
				White:   &PlayerState{ProfileID: "profile-1", Color: ColorWhite, Notation: MoveNotationUCI},
				Outcome: &GameOutcome{Result: ResultBlackWin, Reason: GameOverResignation},
			}},
		},
		{
			"application error",
			`{"type":"error","requestId":"move-1","payload":{"code":"illegal_move","message":"Illegal move"}}`,
			ServerEvent{Type: ServerError, RequestID: "move-1", Error: &ErrorPayload{Code: ErrorIllegalMove, Message: "Illegal move"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeServerEvent([]byte(tt.message))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDecodeEventRejectsMalformedMessages(t *testing.T) {
	tests := []struct{ name, message string }{
		{"missing matchmaking preset", `{"type":"matchmaking.entered","payload":{}}`},
		{"null matchmaking preset", `{"type":"matchmaking.entered","payload":{"timeControl":null}}`},
		{"empty matchmaking preset", `{"type":"matchmaking.entered","payload":{"timeControl":""}}`},
		{"invalid matchmaking color", `{"type":"matchmaking.found","payload":{"gameId":"game-1","color":"purple"}}`},
		{"invalid JSON", `{`},
		{"missing type", `{"payload":{}}`},
		{"missing payload", `{"type":"connection.ready"}`},
		{"null payload", `{"type":"connection.ready","payload":null}`},
		{"array payload", `{"type":"connection.ready","payload":[]}`},
		{"missing profile", `{"type":"connection.ready","payload":{}}`},
		{"empty profile", `{"type":"connection.ready","payload":{"profileId":""}}`},
		{"blank profile", `{"type":"connection.ready","payload":{"profileId":" \t "}}`},
		{"wrong field type", `{"type":"connection.ready","payload":{"profileId":1}}`},
		{"empty game", `{"type":"game.created","payload":{"gameId":"","color":"white"}}`},
		{"invalid color", `{"type":"game.joined","payload":{"gameId":"game-1","color":"purple"}}`},
		{"empty position", `{"type":"game.state","payload":{"gameId":"game-1","fen":"","status":"active","version":0}}`},
		{"invalid status", `{"type":"game.state","payload":{"gameId":"game-1","fen":"position","status":"unknown","version":0}}`},
		{"empty error code", `{"type":"error","payload":{"code":"","message":"error"}}`},
		{"blank error message", `{"type":"error","payload":{"code":"illegal_move","message":" "}}`},
		{"missing version", `{"type":"game.state","payload":{"gameId":"game-1","fen":"position","status":"active"}}`},
		{"null version", `{"type":"game.state","payload":{"gameId":"game-1","fen":"position","status":"active","version":null}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeServerEvent([]byte(tt.message))
			if err == nil {
				t.Fatal("expected malformed message to be rejected")
			}
		})
	}
}
