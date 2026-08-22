package websocket

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"
	"context"
	"errors"
	"sync"
)

type GameSessions struct {
	mu sync.RWMutex

	byGame    map[identity.GameID]map[*Session]struct{}
	bySession map[*Session]identity.GameID
}

func NewGameSessions() *GameSessions {
	return &GameSessions{
		byGame:    make(map[identity.GameID]map[*Session]struct{}),
		bySession: make(map[*Session]identity.GameID),
	}
}

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

func (g *GameSessions) Broadcast(ctx context.Context, gameID identity.GameID, source *Session, message protocol.ServerEnvelope) error {
	g.mu.RLock()

	registered := g.byGame[gameID]
	sessions := make([]*Session, 0, len(registered))

	for session := range registered {
		sessions = append(sessions, session)
	}

	g.mu.RUnlock()

	var sendErrors []error

	for _, session := range sessions {
		outgoing := message

		if session != source {
			outgoing.RequestID = ""
		}

		if err := session.Send(ctx, outgoing); err != nil {
			sendErrors = append(sendErrors, err)
		}
	}

	return errors.Join(sendErrors...)
}

func (g *GameSessions) GameID(session *Session) (identity.GameID, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	gameID, exists := g.bySession[session]
	return gameID, exists
}
