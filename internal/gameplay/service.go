package gameplay

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type Service interface {
	CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error)
	JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error)
	EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error)
	MakeMove(ctx context.Context, command MoveCommand) (MoveResult, error)
	GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error)
}

type GameService struct {
	mu             sync.RWMutex
	waitingPlayers map[timeControlKey]*waitingPlayer
	games          map[identity.GameID]*Game
	pickColor      func() chess.Color
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

	return JoinResult{
		GameID: game.ID,
		Color:  color,
	}, nil
}

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

	waitingResult, currentResult := s.createMatchLocked(waiting, player)

	s.mu.Unlock()

	deliverMatchResult(waiting.result, waitingResult)
	deliverMatchResult(player.result, currentResult)

	return MatchTicket{Result: player.result}, nil
}

// newWaitingPlayer creates a queue entry that is canceled with ctx.
func newWaitingPlayer(ctx context.Context, command EnterMatchmakingCommand) *waitingPlayer {
	return &waitingPlayer{command: command, result: make(chan MatchResult, 1), done: ctx.Done()}
}

// findOpponentLocked matches player within its time-control pool or queues it.
// The caller must hold s.mu.
func (s *GameService) findOpponentLocked(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	s.removeStaleWaitingPlayerLocked()

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

	return waiting, false, nil
}

// removeStaleWaitingPlayerLocked removes canceled players from every pool.
// The caller must hold s.mu.
func (s *GameService) removeStaleWaitingPlayerLocked() {
	for key, waiting := range s.waitingPlayers {
		if !playerDone(waiting.done) {
			continue
		}

		delete(s.waitingPlayers, key)
		close(waiting.result)
	}
}

// createMatchLocked creates and stores a game for two matched players.
// The caller must hold s.mu.
func (s *GameService) createMatchLocked(waiting *waitingPlayer, current *waitingPlayer) (MatchResult, MatchResult) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID := assignMatchmakingColors(waiting.command.ProfileID, current.command.ProfileID, waitingColor)

	gameID := identity.NewGameID()
	timeControl := current.command.timeControl

	game := NewGame(gameID, whiteProfileID, blackProfileID, timeControl.initial, timeControl.increment)

	s.games[gameID] = game

	return MatchResult{GameID: gameID, Color: waitingColor}, MatchResult{GameID: gameID, Color: currentColor}
}

// deliverMatchResult sends one match result and completes its ticket.
func deliverMatchResult(resultChannel chan MatchResult, result MatchResult) {
	resultChannel <- result
	close(resultChannel)
}

// assignMatchmakingColors returns profile IDs ordered as White, then Black.
func assignMatchmakingColors(waitingProfileID identity.ProfileID, currentProfileID identity.ProfileID, waitingColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if waitingColor == chess.White {
		return waitingProfileID, currentProfileID
	}

	return currentProfileID, waitingProfileID
}

// removeWaitingPlayerOnCancel unregisters a queued player when its context ends.
func (s *GameService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	if player.done == nil {
		return
	}

	<-player.done

	s.mu.Lock()
	defer s.mu.Unlock()

	key := player.command.timeControl
	// The player may already have been matched. Only remove it
	// when it is still the exact waiting entry.
	if s.waitingPlayers[key] != player {
		return
	}

	delete(s.waitingPlayers, key)
	close(player.result)
}

// playerDone reports whether a player's cancellation signal has fired.
func playerDone(done <-chan struct{}) bool {
	if done == nil {
		return false
	}

	select {
	case <-done:
		return true
	default:
		return false
	}
}

// MakeMove applies a move to the identified game on behalf of a profile.
func (s *GameService) MakeMove(ctx context.Context, command MoveCommand) (MoveResult, error) {
	if err := ctx.Err(); err != nil {
		return MoveResult{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return MoveResult{}, err
	}

	return game.Move(command)
}

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
