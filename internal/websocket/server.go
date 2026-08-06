package websocket

import (
	"ChessLI/internal/gameplay"
	protocol2 "ChessLI/internal/websocket/protocol"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	maxMessageSize = 64 * 1024
)

// Server accepts and manages WebSocket client connections.
type Server struct {
	httpServer *http.Server

	messageHandler *Handler

	connectionCtx     context.Context
	cancelConnections context.CancelFunc
	connections       sync.WaitGroup
}

// NewServer creates a WebSocket server configured to listen on address.
func NewServer(address string, gameService gameplay.Service) *Server {
	connectionCtx, cancelConnections := context.WithCancel(context.Background())

	server := &Server{
		connectionCtx:     connectionCtx,
		cancelConnections: cancelConnections,
		messageHandler:    NewHandler(gameService),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", server.handleConnection)

	server.httpServer = &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	server.httpServer.RegisterOnShutdown(server.cancelConnections)

	return server
}

// Run serves WebSocket connections until the context is canceled or serving fails.
func (s *Server) Run(ctx context.Context) error {
	serverError := make(chan error, 1)

	go func() {
		serverError <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		s.cancelConnections()
		s.connections.Wait()

		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("run websocket server: %w", err)

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown stops the HTTP server and waits for client connections to close.
func (s *Server) Shutdown(ctx context.Context) error {
	httpError := s.httpServer.Shutdown(ctx)

	connectionsClosed := make(chan struct{})

	go func() {
		s.connections.Wait()
		close(connectionsClosed)
	}()

	select {
	case <-connectionsClosed:
		if httpError != nil {
			return fmt.Errorf("shutdown http server: %w", httpError)
		}

		return nil

	case <-ctx.Done():
		return errors.Join(httpError, fmt.Errorf("wait for websocket connections: %w", ctx.Err()))
	}
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	s.connections.Add(1)
	defer s.connections.Done()

	conn, err := coderws.Accept(w, r, nil)
	if err != nil {
		slog.Error("accept websocket connection", "error", err)
		return
	}
	conn.SetReadLimit(maxMessageSize)

	if err = wsjson.Write(r.Context(), conn, protocol2.ServerEnvelope{Type: protocol2.ServerConnectionReady}); err != nil {
		slog.Error("send connection ack", "error", err)
		_ = conn.CloseNow()
		return
	}

	client := NewSession(conn)

	if err = client.Run(s.connectionCtx, s.messageHandler.Handle); err != nil {
		if isExpectedClose(err) {
			return
		}

		slog.Warn(
			"websocket client disconnected unexpectedly",
			"error",
			err,
		)
	}
}

func isExpectedClose(err error) bool {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	status := coderws.CloseStatus(err)

	return status == coderws.StatusNormalClosure ||
		status == coderws.StatusGoingAway
}
