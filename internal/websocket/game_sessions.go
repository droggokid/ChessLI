package websocket

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	"context"
)

// gameSessions is owned by Handler; access requires its dispatchMu.
type gameSessions struct {
	byGame    map[identity.GameID]map[*Session]struct{}
	bySession map[*Session]identity.GameID // Empty ID means reserved, not yet seated.
}

// newGameSessions returns an empty registry of sessions grouped by game.
func newGameSessions() *gameSessions {
	return &gameSessions{
		byGame:    make(map[identity.GameID]map[*Session]struct{}),
		bySession: make(map[*Session]identity.GameID),
	}
}

// Hold marks an idle session as unavailable while a game operation is in progress.
func (g *gameSessions) Hold(session *Session) error {
	if _, exists := g.bySession[session]; exists {
		return protocol.ErrSessionAlreadyInGame
	}

	g.bySession[session] = ""
	return nil
}

// Release removes a reservation or matchmaking marker without removing game membership.
func (g *gameSessions) Release(session *Session) {
	if g.bySession[session] == "" {
		delete(g.bySession, session)
	}
}

// Add registers a session with one game and rejects conflicting registration.
func (g *gameSessions) Add(gameID identity.GameID, session *Session) error {
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
		if registered.profileID == session.profileID {
			return protocol.ErrSessionAlreadyInGame
		}
	}

	g.byGame[gameID][session] = struct{}{}
	g.bySession[session] = gameID

	return nil
}

// Remove clears a session's registration and returns its game ID, or an empty ID if none.
func (g *gameSessions) Remove(session *Session) identity.GameID {
	gameID := g.bySession[session]
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
func (g *gameSessions) RemoveGame(gameID identity.GameID) {
	for session := range g.byGame[gameID] {
		delete(g.bySession, session)
	}
	delete(g.byGame, gameID)
}

// Broadcast sends a message to the source and the other sessions in a game.
// It returns the source send error after independently attempting peer delivery.
func (g *gameSessions) Broadcast(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) error {
	var sourceErr error
	peerCtx := ctx
	if source != nil {
		sourceErr = source.Send(ctx, message)
		// A committed peer update must survive cancellation of the acting player.
		peerCtx = context.WithoutCancel(ctx)
	}
	g.BroadcastPeers(peerCtx, gameID, source, message)
	return sourceErr
}

// BroadcastPeers queues a message without a request ID for every game session except source.
// Send logs full queues; stopped or canceled peers need no further delivery attempt.
func (g *gameSessions) BroadcastPeers(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) {
	message.RequestID = ""
	for session := range g.byGame[gameID] {
		if session == source {
			continue
		}

		_ = session.Send(ctx, message)
	}
}

// IsConnected reports whether a profile has a session registered to the game.
func (g *gameSessions) IsConnected(
	gameID identity.GameID,
	profileID identity.ProfileID,
) bool {
	for session := range g.byGame[gameID] {
		if session.profileID == profileID {
			return true
		}
	}

	return false
}
