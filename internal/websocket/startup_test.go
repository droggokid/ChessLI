package websocket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	coderws "github.com/coder/websocket"
)

func TestServerShutdownCancelsBlockedConnectionReady(t *testing.T) {
	server := NewServer("", nil)
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

func TestSessionBoundsConnectionReadyWrite(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Sec-WebSocket-Version", "13")
	r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return r
}
