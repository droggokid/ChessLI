package websocket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestIntegrationSessionBoundsConnectionReadyWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		serverConn, peer := net.Pipe()
		defer serverConn.Close()
		defer peer.Close()
		conn, err := coderws.Accept(&pipeResponseWriter{ResponseRecorder: httptest.NewRecorder(), conn: serverConn}, websocketUpgradeRequest(), nil)
		if err != nil {
			t.Fatal(err)
		}
		client := NewSession(identity.NewProfileID(), conn)
		started := time.Now()
		err = client.Run(context.Background(), func(context.Context, *Session, json.RawMessage) error {
			t.Error("handled a command before connection.ready was written")
			return nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("startup error = %v, want deadline exceeded", err)
		}
		if elapsed := time.Since(started); elapsed > 10*time.Second {
			t.Fatalf("startup took %v, want at most 10s", elapsed)
		}
		if err := client.Send(context.Background(), protocol.ServerEnvelope{}); !errors.Is(err, protocol.ErrSessionNotRunning) {
			t.Fatalf("send after failed startup = %v", err)
		}
	})
}

type pipeResponseWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *pipeResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

type observedWriteConn struct {
	net.Conn
	writing chan<- struct{}
}

func (c *observedWriteConn) Write(p []byte) (int, error) {
	select {
	case c.writing <- struct{}{}:
	default:
	}
	return c.Conn.Write(p)
}

func websocketUpgradeRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws?profileId=00000000-0000-4000-8000-000000000001", nil)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Sec-WebSocket-Version", "13")
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return r
}

func TestIntegrationSessionHeartbeatClosesUnresponsiveConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	done := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			done <- err
			return
		}
		session := NewSession(identity.NewProfileID(), conn)
		session.heartbeatInterval = 50 * time.Millisecond
		session.heartbeatTimeout = 50 * time.Millisecond
		done <- session.Run(ctx, func(context.Context, *Session, json.RawMessage) error { return nil })
	}))
	t.Cleanup(server.Close)
	t.Cleanup(cancel)
	peer, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.CloseNow() })
	var ready protocol.ServerEnvelope
	if err := wsjson.Read(ctx, peer, &ready); err != nil || ready.Type != protocol.ServerConnectionReady {
		t.Fatalf("connection.ready = %+v, %v", ready, err)
	}
	// Stop reading so the peer cannot process or answer the heartbeat ping.
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unresponsive session error = %v, want heartbeat deadline", err)
		}
	case <-ctx.Done():
		t.Fatal("heartbeat did not close the unresponsive session")
	}
}
