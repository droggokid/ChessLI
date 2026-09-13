package websocket

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestWebSocketStackMalformedJSONThenGameFlow(t *testing.T) {
	server, url := startStackServer(t)
	defer shutdownStackServer(t, server)

	white := dialStackClient(t, url)
	defer white.CloseNow()
	ready := readStackEnvelope(t, white, protocol.ServerConnectionReady)
	var readyPayload protocol.ConnectionReadyPayload
	decodeStackPayload(t, ready, &readyPayload)
	if readyPayload.ProfileID == "" {
		t.Fatal("connection.ready returned an empty profile ID")
	}

	if err := white.Write(context.Background(), coderws.MessageText, []byte(`{"type":`)); err != nil {
		t.Fatalf("Write(malformed JSON) error = %v", err)
	}
	invalid := readStackEnvelope(t, white, protocol.ServerError)
	if errorCode(t, invalid) != protocol.ErrorInvalidMessage {
		t.Fatalf("malformed JSON error = %+v, want invalid_message", invalid)
	}

	writeStackEnvelope(t, white, protocol.ClientEnvelope{
		Type:      protocol.ClientCreateGame,
		RequestID: "create",
		Payload:   json.RawMessage(`{"color":"white","timeControl":"1+0"}`),
	})
	created := readStackEnvelope(t, white, protocol.ServerGameCreated)
	if created.RequestID != "create" {
		t.Fatalf("game.created request ID = %q, want create", created.RequestID)
	}
	var createPayload protocol.GameCreatedPayload
	decodeStackPayload(t, created, &createPayload)

	black := dialStackClient(t, url)
	defer black.CloseNow()
	readStackEnvelope(t, black, protocol.ServerConnectionReady)
	writeStackEnvelope(t, black, protocol.ClientEnvelope{
		Type:      protocol.ClientJoinGame,
		RequestID: "join",
		Payload:   json.RawMessage(`{"gameId":"` + string(createPayload.GameID) + `"}`),
	})
	readStackEnvelope(t, black, protocol.ServerGameJoined)
	blackInitial := readStackEnvelope(t, black, protocol.ServerGameInitial)
	whiteInitial := readStackEnvelope(t, white, protocol.ServerGameInitial)
	assertStackState(t, blackInitial, 0, "", true)
	assertStackState(t, whiteInitial, 0, "", true)

	writeStackEnvelope(t, white, protocol.ClientEnvelope{
		Type:      protocol.ClientMakeMove,
		RequestID: "move",
		Payload:   json.RawMessage(`{"gameId":"` + string(createPayload.GameID) + `","move":"e2e4","notation":"uci","expectedVersion":0}`),
	})
	whiteState := readStackEnvelope(t, white, protocol.ServerGameState)
	blackState := readStackEnvelope(t, black, protocol.ServerGameState)
	if whiteState.RequestID != "move" || blackState.RequestID != "" {
		t.Fatalf("state request IDs = (%q, %q), want (move, empty)", whiteState.RequestID, blackState.RequestID)
	}
	assertStackState(t, whiteState, 1, "e4", true)
	assertStackState(t, blackState, 1, "e4", true)
}

func TestWebSocketStackShutdownClosesConnection(t *testing.T) {
	server, url := startStackServer(t)
	client := dialStackClient(t, url)
	defer client.CloseNow()
	readStackEnvelope(t, client, protocol.ServerConnectionReady)

	shutdownStackServer(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := client.Read(ctx); err == nil {
		t.Fatal("Read() after Shutdown() succeeded, want closed connection")
	}
}

func TestWebSocketStackReusesProfileFromQuery(t *testing.T) {
	server, endpoint := startStackServer(t)
	defer shutdownStackServer(t, server)

	first := dialStackClient(t, endpoint)
	defer first.CloseNow()
	ready := readStackEnvelope(t, first, protocol.ServerConnectionReady)
	var firstPayload protocol.ConnectionReadyPayload
	decodeStackPayload(t, ready, &firstPayload)

	second := dialStackClient(t, endpoint+"?profileId="+string(firstPayload.ProfileID))
	defer second.CloseNow()
	ready = readStackEnvelope(t, second, protocol.ServerConnectionReady)
	var secondPayload protocol.ConnectionReadyPayload
	decodeStackPayload(t, ready, &secondPayload)
	if secondPayload.ProfileID != firstPayload.ProfileID {
		t.Fatalf("reconnected profile ID = %q, want %q", secondPayload.ProfileID, firstPayload.ProfileID)
	}
}

func TestWebSocketStackBroadcastsDisconnectAndResumeState(t *testing.T) {
	server, endpoint := startStackServer(t)
	defer shutdownStackServer(t, server)

	white, black, gameID, blackProfileID := startStackGame(t, endpoint)
	defer white.CloseNow()
	defer black.CloseNow()

	_ = black.CloseNow()
	disconnected := readStackEnvelope(t, white, protocol.ServerGameState)
	assertStackConnected(t, disconnected, true, false)

	reconnected := dialStackClient(t, endpoint+"?profileId="+string(blackProfileID))
	defer reconnected.CloseNow()
	readStackEnvelope(t, reconnected, protocol.ServerConnectionReady)
	writeStackEnvelope(t, reconnected, protocol.ClientEnvelope{
		Type:      protocol.ClientResumeGame,
		RequestID: "resume",
		Payload:   json.RawMessage(`{"gameId":"` + string(gameID) + `"}`),
	})
	readStackEnvelope(t, reconnected, protocol.ServerGameResumed)
	readStackEnvelope(t, reconnected, protocol.ServerGameInitial)

	connected := readStackEnvelope(t, white, protocol.ServerGameState)
	assertStackConnected(t, connected, true, true)
}

func TestWebSocketStackHeartbeatRemovesUnresponsiveSession(t *testing.T) {
	server, endpoint := startStackServerWithHeartbeat(t, 100*time.Millisecond, 100*time.Millisecond)
	defer shutdownStackServer(t, server)

	white, black, _, _ := startStackGame(t, endpoint)
	defer white.CloseNow()
	defer black.CloseNow()

	disconnected := readStackEnvelope(t, white, protocol.ServerGameState)
	assertStackConnected(t, disconnected, true, false)
}

func startStackServer(t *testing.T) (*Server, string) {
	t.Helper()
	return startStackServerWithHeartbeat(t, heartbeatInterval, heartbeatTimeout)
}

func startStackServerWithHeartbeat(t *testing.T, interval, timeout time.Duration) (*Server, string) {
	t.Helper()
	server := NewServer("", gameplay.NewGameService())
	server.heartbeatInterval = interval
	server.heartbeatTimeout = timeout
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.httpServer.Serve(listener) }()
	return server, "ws://" + listener.Addr().String() + "/ws"
}

func startStackGame(t *testing.T, endpoint string) (*coderws.Conn, *coderws.Conn, identity.GameID, identity.ProfileID) {
	t.Helper()

	white := dialStackClient(t, endpoint)
	readStackEnvelope(t, white, protocol.ServerConnectionReady)
	writeStackEnvelope(t, white, protocol.ClientEnvelope{
		Type:      protocol.ClientCreateGame,
		RequestID: "create",
		Payload:   json.RawMessage(`{"color":"white","timeControl":"1+0"}`),
	})
	created := readStackEnvelope(t, white, protocol.ServerGameCreated)
	var createdPayload protocol.GameCreatedPayload
	decodeStackPayload(t, created, &createdPayload)

	black := dialStackClient(t, endpoint)
	ready := readStackEnvelope(t, black, protocol.ServerConnectionReady)
	var readyPayload protocol.ConnectionReadyPayload
	decodeStackPayload(t, ready, &readyPayload)
	writeStackEnvelope(t, black, protocol.ClientEnvelope{
		Type:      protocol.ClientJoinGame,
		RequestID: "join",
		Payload:   json.RawMessage(`{"gameId":"` + string(createdPayload.GameID) + `"}`),
	})
	readStackEnvelope(t, black, protocol.ServerGameJoined)
	readStackEnvelope(t, black, protocol.ServerGameInitial)
	readStackEnvelope(t, white, protocol.ServerGameInitial)

	return white, black, createdPayload.GameID, readyPayload.ProfileID
}

func shutdownStackServer(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func dialStackClient(t *testing.T, url string) *coderws.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, response, err := coderws.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Dial() status = %d, want %d", response.StatusCode, http.StatusSwitchingProtocols)
	}
	return conn
}

func writeStackEnvelope(t *testing.T, conn *coderws.Conn, message protocol.ClientEnvelope) {
	t.Helper()
	if err := wsjson.Write(context.Background(), conn, message); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func readStackEnvelope(t *testing.T, conn *coderws.Conn, want protocol.ServerMessageType) protocol.ServerEnvelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var message protocol.ServerEnvelope
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if message.Type != want {
		t.Fatalf("message type = %q, want %q", message.Type, want)
	}
	return message
}

func decodeStackPayload(t *testing.T, message protocol.ServerEnvelope, target any) {
	t.Helper()
	raw, err := json.Marshal(message.Payload)
	if err != nil {
		t.Fatalf("Marshal(payload) error = %v", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("Unmarshal(payload) error = %v", err)
	}
}

func errorCode(t *testing.T, message protocol.ServerEnvelope) protocol.ErrorCode {
	t.Helper()
	var payload protocol.ErrorPayload
	decodeStackPayload(t, message, &payload)
	return payload.Code
}

func assertStackState(t *testing.T, message protocol.ServerEnvelope, version uint64, lastMove string, connected bool) {
	t.Helper()
	var payload protocol.GameStatePayload
	decodeStackPayload(t, message, &payload)
	if payload.Version != version || payload.LastMove != lastMove {
		t.Fatalf("state = (version %d, last move %q), want (%d, %q)", payload.Version, payload.LastMove, version, lastMove)
	}
	assertStackConnected(t, message, connected, connected)
}

func assertStackConnected(t *testing.T, message protocol.ServerEnvelope, white, black bool) {
	t.Helper()
	var payload protocol.GameStatePayload
	decodeStackPayload(t, message, &payload)
	if payload.White == nil || payload.Black == nil || payload.White.Connected != white || payload.Black.Connected != black {
		t.Fatalf("connected players = (%+v, %+v), want (%v, %v)", payload.White, payload.Black, white, black)
	}
}
