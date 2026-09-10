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
	bySession map[*Session]sessionRegistration
	changed   map[identity.GameID]chan struct{}
}

type sessionState uint8

const (
	sessionPending sessionState = iota
	sessionInGame
)

type sessionRegistration struct {
	state  sessionState
	gameID identity.GameID
}

// NewGameSessions returns an empty registry of sessions grouped by game.
func NewGameSessions() *GameSessions {
	return &GameSessions{
		byGame:    make(map[identity.GameID]map[*Session]struct{}),
		bySession: make(map[*Session]sessionRegistration),
		changed:   make(map[identity.GameID]chan struct{}),
	}
}

// Reserve holds an idle session while a private game operation is committed.
func (g *GameSessions) Reserve(session *Session) error {
	return g.hold(session)
}

// Queue marks an idle session as waiting for matchmaking.
func (g *GameSessions) Queue(session *Session) error {
	return g.hold(session)
}

func (g *GameSessions) hold(session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.bySession[session]; exists {
		return protocol.ErrSessionAlreadyInGame
	}

	g.bySession[session] = sessionRegistration{state: sessionPending}
	return nil
}

// Release removes a reservation or matchmaking marker without removing game membership.
func (g *GameSessions) Release(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()

	registration, exists := g.bySession[session]
	if exists && registration.state != sessionInGame {
		delete(g.bySession, session)
	}
}

// Add registers a session with one game and rejects conflicting registration.
func (g *GameSessions) Add(gameID identity.GameID, session *Session) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if current, exists := g.bySession[session]; exists {
		if current.state == sessionInGame && current.gameID == gameID {
			return nil
		}
		if current.state == sessionInGame {
			return protocol.ErrSessionAlreadyInGame
		}
	}

	if g.byGame[gameID] == nil {
		g.byGame[gameID] = make(map[*Session]struct{})
	}

	g.byGame[gameID][session] = struct{}{}
	g.bySession[session] = sessionRegistration{state: sessionInGame, gameID: gameID}
	g.signalChangedLocked(gameID)

	return nil
}

// Remove unregisters a session from its game.
func (g *GameSessions) Remove(session *Session) {
	g.mu.Lock()
	defer g.mu.Unlock()

	registration, exists := g.bySession[session]
	if !exists {
		return
	}

	delete(g.bySession, session)
	if registration.state != sessionInGame {
		return
	}

	gameID := registration.gameID
	sessions := g.byGame[gameID]
	delete(sessions, session)
	g.signalChangedLocked(gameID)

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

	registration, exists := g.bySession[session]
	if !exists || registration.state != sessionInGame {
		return "", false
	}

	return registration.gameID, true
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
		if session.profileID == profileID {
			return true
		}
	}

	return false
}
