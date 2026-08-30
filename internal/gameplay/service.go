package gameplay

import (
	"context"
	"math/rand/v2"
	"sync"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=service_mock.go -package=gameplay ChessLI/internal/gameplay Service

type Service interface {
	CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error)
	JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error)
	EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error)
	MakeMove(ctx context.Context, command MoveCommand) (GameSnapshot, error)
	GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error)
}

type GameService struct {
	mu             sync.RWMutex
	waitingPlayers map[timeControlKey]*waitingPlayer
	games          map[identity.GameID]*Game
	pickColor      func() chess.Color
	onGameExpired  func(GameSnapshot)
}

// NewGameService returns an empty, in-memory gameplay service.
func NewGameService() *GameService {
	return &GameService{
		waitingPlayers: make(map[timeControlKey]*waitingPlayer),
		games:          make(map[identity.GameID]*Game),
		pickColor: func() chess.Color {
			if rand.IntN(2) == 0 {
				return chess.White
			}
			return chess.Black
		},
	}
}

func (s *GameService) gameByID(id identity.GameID) (*Game, error) {
	s.mu.RLock()
	game, ok := s.games[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrGameNotFound
	}
	return game, nil
}

// SetGameExpiredHandler sets the function called after a clock expires.
func (s *GameService) SetGameExpiredHandler(handler func(GameSnapshot)) {
	s.mu.Lock()
	s.onGameExpired = handler
	s.mu.Unlock()
}

func (s *GameService) notifyGameExpired(state GameSnapshot) {
	s.mu.RLock()
	handler := s.onGameExpired
	s.mu.RUnlock()
	if handler != nil {
		handler(state)
	}
}

// Close stops every pending game expiration timer.
func (s *GameService) Close() {
	s.mu.RLock()
	games := make([]*Game, 0, len(s.games))
	for _, game := range s.games {
		games = append(games, game)
	}
	s.mu.RUnlock()

	for _, game := range games {
		game.stopExpiration()
	}
}
