package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestIntegrationServerShutdownCancelsBlockedConnectionReady(t *testing.T) {
	server := NewServer("", newTestGameService(t))
	serverConn, peer := net.Pipe()
	defer peer.Close()
	defer serverConn.Close()
	writing := make(chan struct{}, 1)
	w := &pipeResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             &observedWriteConn{Conn: serverConn, writing: writing},
	}
	r := websocketUpgradeRequest()
	done := make(chan struct{})
	go func() {
		server.handleConnection(w, r)
		close(done)
	}()
	t.Cleanup(func() { _ = peer.Close(); <-done })
	select {
	case <-writing:
	case <-time.After(time.Second):
		t.Fatal("connection.ready write did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown did not cancel blocked connection.ready: %v", err)
	}
}

func TestIntegrationConnectionCloseLogs(t *testing.T) {
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
			games := newTestGameService(t)
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
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"?profileId="+string(identity.NewProfileID()), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseNow()
			var ready protocol.ServerEnvelope
			if err := wsjson.Read(ctx, client, &ready); err != nil || ready.Type != protocol.ServerConnectionReady {
				t.Fatalf("connection.ready = %+v, %v", ready, err)
			}
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
