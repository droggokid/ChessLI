package tui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ChessLI/internal/tui/screens/play"
	"ChessLI/internal/websocket/protocol"

	tea "charm.land/bubbletea/v2"
	coderws "github.com/coder/websocket"
)

func TestConnectionCloseCancelsOutstandingDial(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		http.Error(w, "stopped", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	defer close(release)
	connection := newConnection("ws" + strings.TrimPrefix(server.URL, "http"))
	defer connection.Close()
	result := make(chan tea.Msg, 1)
	go func() { result <- connection.connect("", 1)() }()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("dial did not start")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-result:
		if !errors.Is(message.(connectedMsg).err, context.Canceled) {
			t.Fatalf("dial result = %#v", message)
		}
	case <-ctx.Done():
		t.Fatal("dial did not terminate")
	}
}

func TestConnectionCloseJoinsOutstandingReceiveAndDeadline(t *testing.T) {
	reading := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		pingDone := make(chan struct{})
		go func() { defer close(pingDone); reading <- conn.Ping(ctx) }()
		_, _, _ = conn.Read(ctx)
		<-pingDone
	}))
	defer server.Close()
	connection := newConnection("ws" + strings.TrimPrefix(server.URL, "http"))
	defer connection.Close()
	connected := connection.connect("", 1)().(connectedMsg)
	if connected.err != nil {
		t.Fatal(connected.err)
	}
	receiveDone := make(chan tea.Msg, 1)
	deadlineDone := make(chan tea.Msg, 1)
	go func() { receiveDone <- connection.receive(connected.client, 1)() }()
	go func() { deadlineDone <- connection.responseDeadline("request-1")() }()
	select {
	case err := <-reading:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("receive did not process ping")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-receiveDone:
		if message.(receivedMsg).err == nil {
			t.Fatal("closed receive succeeded")
		}
	case <-ctx.Done():
		t.Fatal("receive did not terminate")
	}
	select {
	case <-deadlineDone:
	case <-ctx.Done():
		t.Fatal("deadline did not terminate")
	}
	if err := connection.Close(); err != nil {
		t.Fatal("second close:", err)
	}
	if message := connection.connect("", 2)().(connectedMsg); !errors.Is(message.err, context.Canceled) {
		t.Fatal("closed owner accepted another dial")
	}
}

func TestReconnectClosesPreviousConnectionNormally(t *testing.T) {
	closed := make(chan coderws.StatusCode, 2)
	ctx, network := newTestConnection(t, func(ctx context.Context, conn *coderws.Conn) {
		_, _, err := conn.Read(ctx)
		closed <- coderws.CloseStatus(err)
	})

	result := network.connect("", 2)().(connectedMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	select {
	case status := <-closed:
		if status != coderws.StatusNormalClosure {
			t.Fatalf("reconnect closed previous socket with status %v, want normal closure", status)
		}
	case <-ctx.Done():
		t.Fatal("server did not observe the previous connection closing")
	}
}

func TestSendIncludesExpectedVersionZero(t *testing.T) {
	received := make(chan protocol.ClientEnvelope, 1)
	ctx, network := newTestConnection(t, func(ctx context.Context, conn *coderws.Conn) {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("server read: %v", err)
			return
		}
		if kind != coderws.MessageText {
			t.Error("expected text request")
			return
		}
		var request protocol.ClientEnvelope
		if err := json.Unmarshal(data, &request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		received <- request
	})
	result := network.send(network.client, 1, play.RequestMsg{
		Type: protocol.ClientMakeMove, RequestID: "move-1", Payload: protocol.MovePayload{
			GameID: "game-1", Move: "e2e4", Notation: protocol.MoveNotationUCI, ExpectedVersion: new(uint64(0)),
		},
	})().(sentMsg)
	if result.result.Err != nil {
		t.Fatal(result.result.Err)
	}
	select {
	case request := <-received:
		if request.Type != protocol.ClientMakeMove || request.RequestID != "move-1" {
			t.Fatalf("unexpected envelope: %#v", request)
		}
		move, err := protocol.DecodePayload[protocol.MovePayload](request)
		if err != nil {
			t.Fatal(err)
		}
		if move.ExpectedVersion == nil || *move.ExpectedVersion != 0 {
			t.Fatalf("expected non-nil version zero, got %#v", move)
		}
		if move.GameID != "game-1" || move.Move != "e2e4" || move.Notation != protocol.MoveNotationUCI {
			t.Fatalf("unexpected move: %#v", move)
		}
	case <-ctx.Done():
		t.Fatal("server did not receive request:", ctx.Err())
	}
}

func TestReceiveContinuesAfterUnsupportedAndMalformedMessages(t *testing.T) {
	_, network := newTestConnection(t, func(ctx context.Context, conn *coderws.Conn) {
		if err := conn.Write(ctx, coderws.MessageBinary, []byte("{}")); err != nil {
			t.Errorf("server write: %v", err)
			return
		}
		for _, message := range []string{
			`{"type":"future.event","payload":{}}`,
			`{"type":"connection.ready","payload":null}`,
			`{"type":"connection.ready","payload":{"profileId":"profile-1"}}`,
		} {
			if err := conn.Write(ctx, coderws.MessageText, []byte(message)); err != nil {
				t.Errorf("server write: %v", err)
				return
			}
		}
		_, _, _ = conn.Read(ctx)
	})
	for _, name := range []string{"binary", "unsupported", "malformed"} {
		result := network.receive(network.client, 1)().(receivedMsg)
		if _, ok := errors.AsType[*messageError](result.err); !ok {
			t.Fatalf("%s frame: expected recoverable message error, got %v", name, result.err)
		}
		if name == "unsupported" && !errors.Is(result.err, protocol.ErrUnsupportedEvent) {
			t.Fatalf("expected unsupported event, got %v", result.err)
		}
	}
	result := network.receive(network.client, 1)().(receivedMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.event.Ready == nil || result.event.Ready.ProfileID != "profile-1" {
		t.Fatalf("unexpected ready event: %#v", result.event)
	}
}

func TestConnectionReportsPayloadEncodingFailure(t *testing.T) {
	_, network := newTestConnection(t, func(ctx context.Context, conn *coderws.Conn) {
		_, _, _ = conn.Read(ctx)
	})
	result := network.send(network.client, 1, play.RequestMsg{
		Type: protocol.ClientMakeMove, RequestID: "move-1", Payload: make(chan int),
	})().(sentMsg)
	if result.result.RequestID != "move-1" || result.result.Err == nil ||
		!strings.Contains(result.result.Err.Error(), "encode payload") {
		t.Fatalf("send result = %+v", result)
	}
}

func TestConnectionReportsHandshakeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	network := newConnection("ws" + strings.TrimPrefix(server.URL, "http"))
	defer network.Close()
	if result := network.connect("", 1)().(connectedMsg); result.err == nil {
		t.Fatal("expected handshake failure")
	}
}

func newTestConnection(t *testing.T, handle func(context.Context, *coderws.Conn)) (context.Context, *connection) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		handle(ctx, conn)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(cancel)
	network := newConnection("ws" + strings.TrimPrefix(server.URL, "http"))
	t.Cleanup(func() {
		if err := network.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	})
	if result := network.connect("", 1)().(connectedMsg); result.err != nil {
		t.Fatal(result.err)
	}
	return ctx, network
}
