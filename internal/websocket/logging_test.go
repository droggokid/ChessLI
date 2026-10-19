package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"
	coderws "github.com/coder/websocket"
)

func TestConnectionCloseLogs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		code   coderws.StatusCode
		level  string
		reason string
	}{
		{name: "normal", code: coderws.StatusNormalClosure, level: "INFO", reason: "client_closed"},
		{name: "socket ended", code: -1, level: "INFO", reason: "transport_closed"},
		{name: "policy violation", code: coderws.StatusPolicyViolation, level: "WARN", reason: "error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			games := gameplay.NewGameService()
			defer games.Close()
			server := NewServer("", games)
			finished := make(chan struct{})
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				server.handleConnection(w, r)
			}))
			defer func() {
				server.cancelConnections()
				server.connections.Wait()
				httpServer.Close()
			}()
			client := dialStackClient(t, "ws"+strings.TrimPrefix(httpServer.URL, "http"))
			defer client.CloseNow()
			readStackEnvelope(t, client, protocol.ServerConnectionReady)
			if tt.code == -1 {
				_ = client.CloseNow()
			} else {
				_ = client.Close(tt.code, "")
			}
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("connection handler did not finish")
			}
			decoder := json.NewDecoder(&output)
			for {
				var event map[string]any
				if err := decoder.Decode(&event); err == io.EOF {
					t.Fatal("missing connection.closed log")
				} else if err != nil {
					t.Fatal(err)
				}
				if event["event"] != "connection.closed" {
					continue
				}
				if event["level"] != tt.level || event["reason"] != tt.reason {
					t.Fatalf("close log = %+v", event)
				}
				if _, hasError := event["error"]; hasError != (tt.level == "WARN") {
					t.Fatalf("unexpected error field in close log: %+v", event)
				}
				code, hasCode := event["close_code"]
				if hasCode != (tt.code != -1) || hasCode && code != float64(tt.code) {
					t.Fatalf("unexpected close code in log: %+v", event)
				}
				break
			}
		})
	}
}

func TestFullQueueLogsMetadataWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	client := newQueuedSession("player")
	for len(client.outgoing) < cap(client.outgoing) {
		client.outgoing <- protocol.ServerEnvelope{}
	}
	err := client.Send(context.Background(), protocol.ServerEnvelope{
		Type: protocol.ServerGameState, RequestID: "request", Payload: "private-payload",
	})
	if !errors.Is(err, protocol.ErrSessionQueueFull) {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode queue warning: %v", err)
	}
	if event["event"] != "message.dropped" || event["level"] != "WARN" || event["profile_id"] != "player" ||
		event["request_id"] != "request" || event["message_type"] != string(protocol.ServerGameState) || event["reason"] != "queue_full" {
		t.Fatalf("queue warning = %+v", event)
	}
	if strings.Contains(output.String(), "private-payload") {
		t.Fatal("queue warning included the message payload")
	}
}
