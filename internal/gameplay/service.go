package gameplay

import (
	"context"
	"math/rand/v2"
	"sync"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../websocket/game_service_mock.go -package=websocket -mock_names=Service=MockGameService ChessLI/internal/gameplay Service

// Service is the application boundary offered to transports and other clients.
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

	return CreateResult{GameID: gameID, Color: creatorColor}, nil
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
	return JoinResult{GameID: game.ID, Color: color}, nil
}

// EnterMatchmaking queues a profile or matches it with a compatible opponent.
func (s *GameService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error) {
	if err := ctx.Err(); err != nil {
		return MatchTicket{}, err
	}

	timeControl := command.timeControl
	if err := validateTimeControl(timeControl.initial, timeControl.increment); err != nil {
		return MatchTicket{}, err
	}

	player := newWaitingPlayer(ctx, command)
	s.mu.Lock()
	waiting, queued, err := s.findOpponentLocked(player)
	if err != nil {
		s.mu.Unlock()
		close(player.result)
		return MatchTicket{}, err
	}

	if queued {
		if player.done != nil {
			player.stopCleanup = context.AfterFunc(ctx, func() {
				s.removeWaitingPlayerOnCancel(player)
			})
		}
		s.mu.Unlock()
		return MatchTicket{Result: player.result}, nil
	}

	waitingResult, currentResult, game, err := s.createMatchLocked(waiting, player)
	if err != nil {
		s.mu.Unlock()
		close(waiting.result)
		close(player.result)
		return MatchTicket{}, err
	}
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

func (s *GameService) gameByID(id identity.GameID) (*Game, error) {
	s.mu.RLock()
	game, ok := s.games[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrGameNotFound
	}
	return game, nil
}

func (s *GameService) handleColorPreference(preference ColorPreference) (chess.Color, error) {
	switch preference {
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

// findOpponentLocked matches player within its time-control pool or queues it.
// The caller must hold s.mu.
func (s *GameService) findOpponentLocked(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	s.removeCanceledPlayersLocked()

	for _, waiting := range s.waitingPlayers {
		if waiting.command.ProfileID == player.command.ProfileID {
			return nil, false, ErrAlreadyQueued
		}
	}

	key := player.command.timeControl
	waiting, exists := s.waitingPlayers[key]
	if !exists {
		s.waitingPlayers[key] = player
		return nil, true, nil
	}

	delete(s.waitingPlayers, key)
	stopWaitingPlayerCleanup(waiting)
	return waiting, false, nil
}

// removeCanceledPlayersLocked removes canceled players from every pool.
// The caller must hold s.mu.
func (s *GameService) removeCanceledPlayersLocked() {
	for key, waiting := range s.waitingPlayers {
		if !playerCanceled(waiting.done) {
			continue
		}

		delete(s.waitingPlayers, key)
		stopWaitingPlayerCleanup(waiting)
		close(waiting.result)
	}
}

func (s *GameService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := player.command.timeControl
	if s.waitingPlayers[key] != player {
		return
	}

	delete(s.waitingPlayers, key)
	player.stopCleanup = nil
	close(player.result)
}

// createMatchLocked creates and stores a game for two matched players.
// The caller must hold s.mu.
func (s *GameService) createMatchLocked(waiting, current *waitingPlayer) (MatchResult, MatchResult, *Game, error) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID, err := assignMatchmakingColors(
		waiting.command.ProfileID,
		current.command.ProfileID,
		waitingColor,
	)
	if err != nil {
		return MatchResult{}, MatchResult{}, nil, err
	}

	gameID := identity.NewGameID()
	timeControl := current.command.timeControl
	game := NewGame(gameID, whiteProfileID, blackProfileID, timeControl.initial, timeControl.increment)
	s.games[gameID] = game

	return MatchResult{GameID: gameID, Color: waitingColor},
		MatchResult{GameID: gameID, Color: currentColor},
		game,
		nil
}
