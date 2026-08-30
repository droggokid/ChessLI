package gameplay

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

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
		mu:             sync.RWMutex{},
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

// CreatePrivateGame creates a private game using the requested time control and color preference.
func (s *GameService) CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}

	if err := validateTimeControl(command.Initial, command.Increment); err != nil {
		return CreateResult{}, err
	}

	creatorColor, err := s.handleColorPreference(command.ColorPreference)
	if err != nil {
		return CreateResult{}, err
	}

	whiteProfileID, blackProfileID := assignPrivateColors(command.ProfileID, creatorColor)

	gameID := identity.NewGameID()

	game := NewGame(gameID, whiteProfileID, blackProfileID, command.Initial, command.Increment)

	s.mu.Lock()
	s.games[gameID] = game
	s.mu.Unlock()

	return CreateResult{
		GameID: gameID,
		Color:  creatorColor,
	}, nil
}

func validateTimeControl(initial, increment time.Duration) error {
	if initial <= 0 || increment < 0 {
		return ErrInvalidTimeControl
	}

	return nil
}

func (s *GameService) handleColorPreference(cp ColorPreference) (chess.Color, error) {
	switch cp {
	case ColorWhite:
		return chess.White, nil
	case ColorBlack:
		return chess.Black, nil
	case ColorRandom:
		return s.pickColor(), nil
	default:
		return chess.NoColor, ErrInvalidColorPreference
	}
}

func assignPrivateColors(profileID identity.ProfileID, creatorColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	var (
		whiteProfileID identity.ProfileID
		blackProfileID identity.ProfileID
	)

	if creatorColor == chess.White {
		whiteProfileID = profileID
	} else {
		blackProfileID = profileID
	}

	return whiteProfileID, blackProfileID
}

// JoinPrivateGame seats a profile in the open position of a private game.
func (s *GameService) JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error) {
	if err := ctx.Err(); err != nil {
		return JoinResult{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return JoinResult{}, err
	}

	color, err := game.JoinPrivate(command)
	if err != nil {
		return JoinResult{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)

	return JoinResult{
		GameID: game.ID,
		Color:  color,
	}, nil
}

// EnterMatchmaking queues a profile or matches it with a compatible opponent.
func (s *GameService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error) {
	if err := ctx.Err(); err != nil {
		return MatchTicket{}, err
	}

	player := newWaitingPlayer(ctx, command)

	timeControl := command.timeControl

	if err := validateTimeControl(timeControl.initial, timeControl.increment); err != nil {
		return MatchTicket{}, err
	}

	s.mu.Lock()

	waiting, queued, err := s.findOpponentLocked(player)
	if err != nil {
		s.mu.Unlock()
		close(player.result)

		return MatchTicket{}, err
	}

	if queued {
		s.mu.Unlock()

		go s.removeWaitingPlayerOnCancel(player)

		return MatchTicket{Result: player.result}, nil
	}

	waitingResult, currentResult, game := s.createMatchLocked(waiting, player)

	s.mu.Unlock()

	game.scheduleExpiration(s.notifyGameExpired)

	deliverMatchResult(waiting.result, waitingResult)
	deliverMatchResult(player.result, currentResult)

	return MatchTicket{Result: player.result}, nil
}

// MakeMove applies a move to the identified game on behalf of a profile.
func (s *GameService) MakeMove(ctx context.Context, command MoveCommand) (GameSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}

	state, err := game.Move(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)

	return state, nil
}

// GameState returns the current authoritative snapshot of a game.
func (s *GameService) GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(gameID)
	if err != nil {
		return GameSnapshot{}, err
	}

	return game.Snapshot(), nil
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
