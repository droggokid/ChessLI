package websocket

import (
	"ChessLI/internal/identity"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"ChessLI/internal/gameplay"

	coderws "github.com/coder/websocket"
)

const (
	maxMessageSize    = 64 * 1024
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 10 * time.Second
)

// Server accepts and manages WebSocket client connections.
// Run must be called at most once.
type Server struct {
	httpServer *http.Server

	messageHandler *Handler
	gameService    gameplay.Service

	connectionCtx     context.Context
	cancelConnections context.CancelFunc
	connections       sync.WaitGroup

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewServer creates a WebSocket server configured to listen on address.
func NewServer(address string, gameService gameplay.Service) *Server {
	connectionCtx, cancelConnections := context.WithCancel(context.Background())

	sessions := newGameSessions()

	server := &Server{
		connectionCtx:     connectionCtx,
		messageHandler:    newHandler(gameService, sessions),
		gameService:       gameService,
		cancelConnections: cancelConnections,
		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", handleHealth)
	mux.HandleFunc("GET /ws", server.handleConnection)

	server.httpServer = &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	server.httpServer.RegisterOnShutdown(server.cancelConnections)

	return server
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
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
		s.messageHandler.Close()

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

// Shutdown cancels connections and joins them and their match waiters, bounded by ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	httpError := s.httpServer.Shutdown(ctx)

	connectionsClosed := make(chan struct{})

	go func() {
		s.connections.Wait()
		s.messageHandler.Close()
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

	profileID := identity.NewProfileID()
	if rawProfileID := r.URL.Query().Get("profileId"); rawProfileID != "" {
		var err error
		profileID, err = identity.ParseProfileID(rawProfileID)
		if err != nil {
			http.Error(w, "invalid profile ID", http.StatusBadRequest)
			return
		}
	}

	conn, err := coderws.Accept(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade rejected", "event", "connection.rejected",
			"profile_id", profileID, "reason", "upgrade_failed", "error", err)
		return
	}
	conn.SetReadLimit(maxMessageSize)

	client := NewSession(profileID, conn)
	client.activeGameID = s.gameService.ActiveGame(profileID)
	client.heartbeatInterval = s.heartbeatInterval
	client.heartbeatTimeout = s.heartbeatTimeout
	defer s.messageHandler.Disconnect(s.connectionCtx, client)

	started := time.Now()
	slog.Info("websocket connection opened", "event", "connection.opened", "profile_id", profileID)
	err = client.Run(s.connectionCtx, s.messageHandler.Handle)
	level, reason := slog.LevelInfo, "client_closed"
	switch {
	case s.connectionCtx.Err() != nil:
		reason = "shutdown"
	case errors.Is(err, context.DeadlineExceeded):
		level, reason = slog.LevelWarn, "timeout"
	case errors.Is(err, io.EOF):
		reason = "transport_closed"
	case err != nil && !isExpectedClose(err):
		level, reason = slog.LevelWarn, "error"
	}
	attributes := []any{"event", "connection.closed",
		"profile_id", profileID, "duration_ms", time.Since(started).Milliseconds(),
		"reason", reason}
	if code := coderws.CloseStatus(err); code != -1 {
		attributes = append(attributes, "close_code", int(code))
	}
	if level >= slog.LevelWarn {
		attributes = append(attributes, "error", err)
	}
	slog.Log(s.connectionCtx, level, "websocket connection closed", attributes...)
}

// BroadcastGameState sends an unsolicited authoritative state to every session in the game.
func (s *Server) BroadcastGameState(state gameplay.GameSnapshot) {
	s.messageHandler.Publish(s.connectionCtx, state)
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
