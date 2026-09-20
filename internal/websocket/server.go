package websocket

import (
	"ChessLI/internal/identity"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/corentings/chess/v2"
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
	gameSessions   *GameSessions

	connectionCtx     context.Context
	cancelConnections context.CancelFunc
	connections       sync.WaitGroup

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewServer creates a WebSocket server configured to listen on address.
func NewServer(address string, gameService gameplay.Service) *Server {
	connectionCtx, cancelConnections := context.WithCancel(context.Background())

	sessions := NewGameSessions()

	server := &Server{
		connectionCtx:     connectionCtx,
		messageHandler:    NewHandler(gameService, sessions),
		gameSessions:      sessions,
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

	profile := identity.NewProfile()
	if rawProfileID := r.URL.Query().Get("profileId"); rawProfileID != "" {
		var err error
		profile, err = identity.ParseProfile(rawProfileID)
		if err != nil {
			http.Error(w, "invalid profile ID", http.StatusBadRequest)
			return
		}
	}

	conn, err := coderws.Accept(w, r, nil)
	if err != nil {
		slog.Error("accept websocket connection", "error", err)
		return
	}
	conn.SetReadLimit(maxMessageSize)

	if err = wsjson.Write(r.Context(), conn, protocol.ServerEnvelope{
		Type: protocol.ServerConnectionReady,
		Payload: protocol.ConnectionReadyPayload{
			ProfileID: profile.ID,
		},
	}); err != nil {
		slog.Error("send connection ack", "error", err)
		_ = conn.CloseNow()
		return
	}

	client := NewSession(profile, conn)
	client.heartbeatInterval = s.heartbeatInterval
	client.heartbeatTimeout = s.heartbeatTimeout
	defer s.removeSession(client)

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

func (s *Server) removeSession(client *Session) {
	gameID := s.gameSessions.Remove(client)
	if gameID == "" {
		return
	}

	state, err := s.messageHandler.gameService.GameState(s.connectionCtx, gameID)
	if err != nil {
		slog.Warn("fetch disconnected game state", "game_id", gameID, "error", err)
		return
	}
	s.BroadcastGameState(state)
}

// BroadcastGameState sends an unsolicited authoritative state to every session in the game.
func (s *Server) BroadcastGameState(state gameplay.GameSnapshot) {
	envelope := protocol.ServerEnvelope{
		Type:    protocol.ServerGameState,
		Payload: s.messageHandler.gameStatePayload(state),
	}

	if err := s.gameSessions.Broadcast(s.connectionCtx, state.GameID, nil, envelope); err != nil {
		slog.Warn("broadcast automatic game state", "game_id", state.GameID, "error", err)
	}
	if state.Outcome != chess.NoOutcome {
		s.gameSessions.RemoveGame(state.GameID)
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
