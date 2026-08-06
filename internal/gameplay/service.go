package gameplay

import (
	"context"
	"math/rand/v2"
	"sync"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type Service interface {
	CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error)
	JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error)
	EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error)
	MakeMove(ctx context.Context, command MoveCommand) (MoveResult, error)
}

type MemoryService struct {
	mu            sync.RWMutex
	waitingPlayer *waitingPlayer
	games         map[identity.GameID]*Game
	pickColor     func() chess.Color
}

// NewMemoryService returns an empty, in-memory gameplay service.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		mu:    sync.RWMutex{},
		games: make(map[identity.GameID]*Game),
		pickColor: func() chess.Color {
			if rand.IntN(2) == 0 {
				return chess.White
			}

			return chess.Black
		},
	}
}

func (s *MemoryService) gameByID(id identity.GameID) (*Game, error) {
	s.mu.RLock()
	game, ok := s.games[id]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrGameNotFound
	}

	return game, nil
}

// CreatePrivateGame creates a private game using the requested time control and color preference.
func (s *MemoryService) CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (CreateResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}

	if err := validateTimeControl(&command); err != nil {
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

func validateTimeControl(command *CreatePrivateCommand) error {
	if command.Initial <= 0 {
		return ErrInvalidTimeControl
	}

	if command.Increment < 0 {
		return ErrInvalidTimeControl
	}
	return nil
}

func (s *MemoryService) handleColorPreference(cp ColorPreference) (chess.Color, error) {
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
func (s *MemoryService) JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (JoinResult, error) {
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

func (s *MemoryService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error) {
	if err := ctx.Err(); err != nil {
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

func newWaitingPlayer(ctx context.Context, command EnterMatchmakingCommand) *waitingPlayer {
	return &waitingPlayer{command: command, result: make(chan MatchResult, 1), done: ctx.Done()}
}

func (s *MemoryService) findOpponentLocked(player *waitingPlayer) (opponent *waitingPlayer, queued bool, err error) {
	s.removeStaleWaitingPlayerLocked()

	if s.waitingPlayer == nil {
		s.waitingPlayer = player
		return nil, true, nil
	}

	waiting := s.waitingPlayer

	if waiting.command.ProfileID == player.command.ProfileID {
		return nil, false, ErrAlreadyQueued
	}

	if !compatibleTimeControls(waiting.command, player.command) {
		return nil, false, ErrNoCompatibleOpponent
	}

	s.waitingPlayer = nil

	return waiting, false, nil
}

func (s *MemoryService) removeStaleWaitingPlayerLocked() {
	if s.waitingPlayer == nil {
		return
	}

	if !playerDone(s.waitingPlayer.done) {
		return
	}

	stale := s.waitingPlayer
	s.waitingPlayer = nil

	close(stale.result)
}

func compatibleTimeControls(first EnterMatchmakingCommand, second EnterMatchmakingCommand) bool {
	return first.Initial == second.Initial &&
		first.Increment == second.Increment
}

func (s *MemoryService) createMatchLocked(waiting *waitingPlayer, current *waitingPlayer) (MatchResult, MatchResult) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID := assignMatchmakingColors(waiting.command.ProfileID, current.command.ProfileID, waitingColor)

	gameID := identity.NewGameID()

	game := NewGame(gameID, whiteProfileID, blackProfileID, current.command.Initial, current.command.Increment)

	s.games[gameID] = game

	return MatchResult{GameID: gameID, Color: waitingColor}, MatchResult{GameID: gameID, Color: currentColor}
}

func deliverMatchResult(resultChannel chan MatchResult, result MatchResult) {
	resultChannel <- result
	close(resultChannel)
}

func assignMatchmakingColors(waitingProfileID identity.ProfileID, currentProfileID identity.ProfileID, waitingColor chess.Color) (identity.ProfileID, identity.ProfileID) {
	if waitingColor == chess.White {
		return waitingProfileID, currentProfileID
	}

	return currentProfileID, waitingProfileID
}

func (s *MemoryService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	if player.done == nil {
		return
	}

	<-player.done

	s.mu.Lock()
	defer s.mu.Unlock()

	// The player may already have been matched. Only remove it
	// when it is still the exact waiting entry.
	if s.waitingPlayer != player {
		return
	}

	s.waitingPlayer = nil
	close(player.result)
}

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
func (s *MemoryService) MakeMove(ctx context.Context, command MoveCommand) (MoveResult, error) {
	if err := ctx.Err(); err != nil {
		return MoveResult{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return MoveResult{}, err
	}

	return game.Move(command)
}
