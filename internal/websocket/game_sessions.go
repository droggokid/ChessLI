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
	changed   map[identity.GameID]chan struct{}
}

// NewGameSessions returns an empty registry of sessions grouped by game.
func NewGameSessions() *GameSessions {
	return &GameSessions{
		byGame:    make(map[identity.GameID]map[*Session]struct{}),
		bySession: make(map[*Session]identity.GameID),
		changed:   make(map[identity.GameID]chan struct{}),
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
	g.signalChangedLocked(gameID)

	return nil
}

// Remove unregisters a session from its game and returns that game's ID.
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
	g.signalChangedLocked(gameID)

	if len(sessions) == 0 {
		delete(g.byGame, gameID)
	}

	return gameID
}

// Broadcast sends a message to the source and the other sessions in a game.
// Source delivery is required; peer delivery is best effort without a request ID.
func (g *GameSessions) Broadcast(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) error {
	g.mu.RLock()

	registered := g.byGame[gameID]
	sessions := make([]*Session, 0, len(registered))

	for session := range registered {
		sessions = append(sessions, session)
	}

	g.mu.RUnlock()

	if source != nil {
		if err := source.Send(ctx, message); err != nil {
			return err
		}
	}

	for _, session := range sessions {
		if session == source {
			continue
		}

		outgoing := message
		outgoing.RequestID = ""

		if err := session.Send(ctx, outgoing); err != nil {
			slog.Warn("broadcast to game session", "game_id", gameID, "error", err)
		}
	}

	return nil
}

// BroadcastPeers sends a message to every game session except source.
func (g *GameSessions) BroadcastPeers(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) {
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

// WaitForPlayers waits until count sessions have registered with a game.
func (g *GameSessions) WaitForPlayers(ctx context.Context, gameID identity.GameID, count int) error {
	for {
		g.mu.Lock()
		if len(g.byGame[gameID]) >= count {
			g.mu.Unlock()
			return nil
		}

		changed := g.changed[gameID]
		if changed == nil {
			changed = make(chan struct{})
			g.changed[gameID] = changed
		}
		g.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (g *GameSessions) signalChangedLocked(gameID identity.GameID) {
	if changed := g.changed[gameID]; changed != nil {
		close(changed)
		delete(g.changed, gameID)
	}
}

// IsConnected reports whether a profile has an active session registered to the game.
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
