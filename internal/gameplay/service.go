package gameplay

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../gameplay/service_mock.go -package=gameplay -mock_names=Service=MockGameService ChessLI/internal/gameplay Service

// Service is the application boundary offered to transports and other clients.
// Implementations must support concurrent calls. Synchronous in-memory operations
// finish once admitted; context cancellation controls queued matchmaking work.
type Service interface {
	CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (GameAssignment, error)
	JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (GameAssignment, error)
	ResumeGame(context.Context, ResumeGameCommand) (GameSnapshot, error)
	EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error)
	MakeMove(ctx context.Context, command MoveCommand) (GameSnapshot, error)
	GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error)
	Resign(context.Context, ResignCommand) (GameSnapshot, error)
	OfferDraw(context.Context, OfferDrawCommand) (GameSnapshot, error)
	AcceptDraw(context.Context, DrawOfferResponseCommand) (GameSnapshot, error)
	DeclineDraw(context.Context, DrawOfferResponseCommand) (GameSnapshot, error)
}

// GameService is an in-memory Service safe for concurrent use by multiple goroutines.
type GameService struct {
	// mu guards service state. Helpers ending in Locked require callers to hold it and do not lock it.
	mu            sync.RWMutex
	queue         matchQueue
	games         map[identity.GameID]*game
	pickColor     func() chess.Color // Always White or Black.
	onGameExpired func(GameSnapshot)
	notifications sync.WaitGroup
	closed        bool
}

// NewGameService returns an empty, in-memory gameplay service.
func NewGameService() *GameService {
	return &GameService{
		queue: make(matchQueue),
		games: make(map[identity.GameID]*game),
		pickColor: func() chess.Color {
			if rand.IntN(2) == 0 {
				return chess.White
			}
			return chess.Black
		},
	}
}

// SetGameExpiredHandler sets the handler called asynchronously when a game clock expires.
// The handler may be called concurrently and must return. Close waits for it.
func (s *GameService) SetGameExpiredHandler(handler func(GameSnapshot)) {
	s.mu.Lock()
	s.onGameExpired = handler
	s.mu.Unlock()
}

func (s *GameService) notifyGameExpired(state GameSnapshot) {
	s.mu.Lock()
	handler := s.onGameExpired
	if handler == nil || s.closed {
		s.mu.Unlock()
		return
	}
	s.notifications.Add(1)
	s.mu.Unlock()
	// Notifications may call back into transport code that is waiting on this operation.
	go func() {
		defer s.notifications.Done()
		handler(state)
	}()
}

// CreatePrivateGame creates a private game using the requested time control and color preference.
func (s *GameService) CreatePrivateGame(_ context.Context, command CreatePrivateCommand) (GameAssignment, error) {
	if err := validateProfileID(command.ProfileID); err != nil {
		return GameAssignment{}, err
	}
	if err := validateTimeControl(command.Initial, command.Increment); err != nil {
		return GameAssignment{}, err
	}

	creatorColor, err := s.handleColorPreference(command.ColorPreference)
	if err != nil {
		return GameAssignment{}, err
	}

	whiteProfileID, blackProfileID := assignColors(command.ProfileID, "", creatorColor)
	gameID := identity.NewGameID()
	game := newGame(gameID, whiteProfileID, blackProfileID, command.Initial, command.Increment)

	s.mu.Lock()
	s.games[gameID] = game
	s.mu.Unlock()

	slog.Info("private game created", "event", "game.created", "game_id", gameID,
		"profile_id", command.ProfileID, "mode", "private",
		"initial_ms", command.Initial.Milliseconds(), "increment_ms", command.Increment.Milliseconds())
	return GameAssignment{GameID: gameID, Color: creatorColor}, nil
}

// JoinPrivateGame seats a profile in the open position of a private game.
func (s *GameService) JoinPrivateGame(_ context.Context, command JoinPrivateCommand) (GameAssignment, error) {
	if err := validateProfileID(command.ProfileID); err != nil {
		return GameAssignment{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameAssignment{}, err
	}

	color, err := game.joinPrivate(command)
	if err != nil {
		return GameAssignment{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)
	return GameAssignment{GameID: command.GameID, Color: color}, nil
}

// ResumeGame returns the current state, rejecting profiles that are not participants.
func (s *GameService) ResumeGame(_ context.Context, command ResumeGameCommand) (GameSnapshot, error) {

	if err := validateProfileID(command.ProfileID); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	state, err := game.resume(command.ProfileID)
	if err == nil {
		slog.Info("game resumed", "event", "game.resumed", "game_id", command.GameID,
			"profile_id", command.ProfileID, "outcome", state.Outcome, "version", state.Version)
	}
	return state, err
}

// EnterMatchmaking queues a profile or matches it with an opponent using the same time control.
// Canceling ctx removes an unmatched profile from the queue.
func (s *GameService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error) {
	if err := ctx.Err(); err != nil {
		return MatchTicket{}, err
	}
	if err := validateProfileID(command.ProfileID); err != nil {
		return MatchTicket{}, err
	}

	if err := validateTimeControl(command.Initial, command.Increment); err != nil {
		return MatchTicket{}, err
	}

	player := newWaitingPlayer(ctx, command)
	s.mu.Lock()
	waiting, queued, err := s.queue.findOpponent(player)
	if err != nil {
		s.mu.Unlock()
		return MatchTicket{}, err
	}

	if queued {
		if player.done != nil {
			player.stopCleanup = context.AfterFunc(ctx, func() {
				s.removeWaitingPlayerOnCancel(player)
			})
		}
		slog.Info("player queued", "event", "matchmaking.queued", "profile_id", command.ProfileID,
			"initial_ms", command.Initial.Milliseconds(), "increment_ms", command.Increment.Milliseconds())
		s.mu.Unlock()
		return MatchTicket{Result: player.result}, nil
	}

	waitingResult, currentResult, game := s.createMatchLocked(waiting, player)
	s.mu.Unlock()

	game.scheduleExpiration(s.notifyGameExpired)
	deliverMatchResult(waiting.result, waitingResult)
	deliverMatchResult(player.result, currentResult)
	return MatchTicket{Result: player.result}, nil
}

func (s *GameService) removeWaitingPlayerOnCancel(player *waitingPlayer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.removeCanceledPlayer(player)
}

// createMatchLocked creates and stores a game for two matched players.
// The caller must hold s.mu.
func (s *GameService) createMatchLocked(waiting, current *waitingPlayer) (GameAssignment, GameAssignment, *game) {
	waitingColor := s.pickColor()
	currentColor := waitingColor.Other()

	whiteProfileID, blackProfileID := assignColors(
		waiting.command.ProfileID,
		current.command.ProfileID,
		waitingColor,
	)
	gameID := identity.NewGameID()
	game := newGame(gameID, whiteProfileID, blackProfileID, current.command.Initial, current.command.Increment)
	s.games[gameID] = game
	slog.Info("game started", "event", "game.started", "game_id", gameID, "mode", "matchmaking",
		"white_profile_id", whiteProfileID, "black_profile_id", blackProfileID,
		"initial_ms", current.command.Initial.Milliseconds(), "increment_ms", current.command.Increment.Milliseconds())

	return GameAssignment{GameID: gameID, Color: waitingColor},
		GameAssignment{GameID: gameID, Color: currentColor},
		game
}

// MakeMove applies a move to the identified game on behalf of a profile.
func (s *GameService) MakeMove(_ context.Context, command MoveCommand) (GameSnapshot, error) {

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	state, err := game.move(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	game.scheduleExpiration(s.notifyGameExpired)
	return state, nil
}

// GameState returns the current snapshot, settling an expired clock before returning.
func (s *GameService) GameState(_ context.Context, gameID identity.GameID) (GameSnapshot, error) {

	game, err := s.gameByID(gameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	state := game.snapshot()
	game.notifyExpiration(s.notifyGameExpired)
	return state, nil
}

// Resign resigns a game on behalf of a participant.
func (s *GameService) Resign(_ context.Context, command ResignCommand) (GameSnapshot, error) {

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	snapshot, err := game.resign(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	game.stopExpiration()
	return snapshot, nil
}

// OfferDraw makes a draw offer on behalf of a game participant.
func (s *GameService) OfferDraw(_ context.Context, command OfferDrawCommand) (GameSnapshot, error) {

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	return game.offerDraw(command)
}

// AcceptDraw accepts a pending draw offer on behalf of a game participant.
func (s *GameService) AcceptDraw(_ context.Context, command DrawOfferResponseCommand) (GameSnapshot, error) {

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	snapshot, err := game.acceptDraw(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	game.stopExpiration()
	return snapshot, nil
}

// DeclineDraw declines a pending draw offer on behalf of a game participant.
func (s *GameService) DeclineDraw(_ context.Context, command DrawOfferResponseCommand) (GameSnapshot, error) {

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer game.notifyExpiration(s.notifyGameExpired)

	return game.declineDraw(command)
}

// Close stops pending expiration timers and waits for notification handlers.
// Call it after application operations have stopped, never from a notification handler.
func (s *GameService) Close() {
	s.mu.Lock()
	s.closed = true
	games := make([]*game, 0, len(s.games))
	for _, game := range s.games {
		games = append(games, game)
	}
	s.mu.Unlock()

	for _, game := range games {
		game.stopExpiration()
	}
	s.notifications.Wait()
}

func (s *GameService) gameByID(id identity.GameID) (*game, error) {
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
