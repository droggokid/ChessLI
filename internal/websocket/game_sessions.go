package websocket

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	"context"
	"log/slog"
	"sync"
)

// GameSessions tracks sessions by game and is safe for concurrent use.
type GameSessions struct {
	mu sync.RWMutex

	byGame    map[identity.GameID]map[*Session]struct{}
	bySession map[*Session]identity.GameID
}

// NewGameSessions returns an empty registry of sessions grouped by game.
func NewGameSessions() *GameSessions {
	return &GameSessions{
		byGame:    make(map[identity.GameID]map[*Session]struct{}),
		bySession: make(map[*Session]identity.GameID),
	}
}

// Hold marks an idle session as unavailable while a game operation is in progress.
func (g *GameSessions) Hold(session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.bySession[session]; exists {
		return protocol.ErrSessionAlreadyInGame
	}

	g.bySession[session] = ""
	return nil
}

// Release removes a reservation or matchmaking marker without removing game membership.
func (g *GameSessions) Release(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()

	gameID, exists := g.bySession[session]
	if exists && gameID == "" {
		delete(g.bySession, session)
	}
}

// Add registers a session with one game and rejects conflicting registration.
func (g *GameSessions) Add(gameID identity.GameID, session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if sessionRunState(session.runState.Load()) != sessionRunning {
		return protocol.ErrSessionNotRunning
	}

	if currentGameID, exists := g.bySession[session]; exists {
		if currentGameID == gameID {
			return nil
		}
		if currentGameID != "" {
			return protocol.ErrSessionAlreadyInGame
		}
	}

	if g.byGame[gameID] == nil {
		g.byGame[gameID] = make(map[*Session]struct{})
	}
	for registered := range g.byGame[gameID] {
		if registered != session && registered.profile.ID == session.profile.ID {
			return protocol.ErrSessionAlreadyInGame
		}
	}

	g.byGame[gameID][session] = struct{}{}
	g.bySession[session] = gameID

	return nil
}

// Remove clears a session's registration and returns its game ID, or an empty ID if none.
func (g *GameSessions) Remove(session *Session) identity.GameID {
	g.mu.Lock()
	defer g.mu.Unlock()

	gameID, exists := g.bySession[session]
	if !exists {
		return ""
	}

	delete(g.bySession, session)
	if gameID == "" {
		return ""
	}

	sessions := g.byGame[gameID]
	delete(sessions, session)

	if len(sessions) == 0 {
		delete(g.byGame, gameID)
	}

	return gameID
}

// RemoveGame unregisters every session from gameID.
func (g *GameSessions) RemoveGame(gameID identity.GameID) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for session := range g.byGame[gameID] {
		delete(g.bySession, session)
	}
	delete(g.byGame, gameID)
}

// Broadcast sends a message to the source and the other sessions in a game.
// It returns the source send error after independently attempting peer delivery.
func (g *GameSessions) Broadcast(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) error {
	var sourceErr error
	peerCtx := ctx
	if source != nil {
		sourceErr = source.Send(ctx, message)
		// Peer enqueues are finite, non-blocking effects of the committed action.
		// They must survive cancellation of the acting player's operation.
		peerCtx = context.WithoutCancel(ctx)
	}
	g.BroadcastPeers(peerCtx, gameID, source, message)
	return sourceErr
}

// BroadcastPeers queues a message without a request ID for every game session except source.
// Delivery errors are logged.
func (g *GameSessions) BroadcastPeers(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) {
	message.RequestID = ""
	g.mu.RLock()

	registered := g.byGame[gameID]
	sessions := make([]*Session, 0, len(registered))
	for session := range registered {
		sessions = append(sessions, session)
	}

	g.mu.RUnlock()

	for _, session := range sessions {
		if session == source {
			continue
		}

		if err := session.Send(ctx, message); err != nil {
			slog.Warn("broadcast to game session", "game_id", gameID, "error", err)
		}
	}
}

// IsConnected reports whether a profile has a session registered to the game.
func (g *GameSessions) IsConnected(
	gameID identity.GameID,
	profileID identity.ProfileID,
) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for session := range g.byGame[gameID] {
		if session.profile.ID == profileID {
			return true
		}
	}

	return false
}
