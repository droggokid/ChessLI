package websocket

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	"context"
	"log/slog"
	"sync"
)

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

// Add registers a session with one game and rejects conflicting registration.
func (g *GameSessions) Add(gameID identity.GameID, session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if currentGameID, exists := g.bySession[session]; exists {
		if currentGameID == gameID {
			return nil
		}

		return protocol.ErrSessionAlreadyInGame
	}

	if g.byGame[gameID] == nil {
		g.byGame[gameID] = make(map[*Session]struct{})
	}

	g.byGame[gameID][session] = struct{}{}
	g.bySession[session] = gameID

	return nil
}

// Remove unregisters a session from its game.
func (g *GameSessions) Remove(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()

	gameID, exists := g.bySession[session]
	if !exists {
		return
	}

	delete(g.bySession, session)

	sessions := g.byGame[gameID]
	delete(sessions, session)

	if len(sessions) == 0 {
		delete(g.byGame, gameID)
	}
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

// GameID returns the game currently associated with a session.
func (g *GameSessions) GameID(session *Session) (identity.GameID, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	gameID, exists := g.bySession[session]
	return gameID, exists
}

// IsConnected reports whether a profile has an active session registered to the game.
func (g *GameSessions) IsConnected(
	gameID identity.GameID,
	profileID identity.ProfileID,
) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for session := range g.byGame[gameID] {
		if session.profileID == profileID {
			return true
		}
	}

	return false
}
