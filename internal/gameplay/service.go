package gameplay

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
	"ChessLI/internal/persistence"

	"github.com/corentings/chess/v2"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../gameplay/service_mock.go -package=gameplay -mock_names=Service=MockGameService ChessLI/internal/gameplay Service

// Service is the application boundary offered to transports and other clients.
// Implementations must support concurrent calls. Synchronous in-memory operations
// finish once admitted. Canceled matchmaking contexts prevent pairing; callers
// remove abandoned entries with CancelMatchmaking.
type Service interface {
	ValidateProfile(context.Context, identity.ProfileID) error
	ActiveGame(profileID identity.ProfileID) identity.GameID
	CreatePrivateGame(ctx context.Context, command CreatePrivateCommand) (GameAssignment, error)
	JoinPrivateGame(ctx context.Context, command JoinPrivateCommand) (GameAssignment, error)
	ResumeGame(context.Context, ResumeGameCommand) (GameSnapshot, error)
	EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchmakingResult, error)
	CancelMatchmaking(entry *MatchmakingEntry)
	MakeMove(ctx context.Context, command MoveCommand) (GameSnapshot, error)
	GameState(ctx context.Context, gameID identity.GameID) (GameSnapshot, error)
	Resign(context.Context, ResignCommand) (GameSnapshot, error)
	OfferDraw(context.Context, OfferDrawCommand) (GameSnapshot, error)
	AcceptDraw(context.Context, DrawOfferResponseCommand) (GameSnapshot, error)
	DeclineDraw(context.Context, DrawOfferResponseCommand) (GameSnapshot, error)
}

// GameService is an in-memory Service safe for concurrent use by multiple goroutines.
type GameService struct {
	// mu protects games, matchmaking, timers, and notification registration.
	// Public methods and background callbacks acquire it; private helpers use it
	// through their caller. Notification handlers run asynchronously.
	mu            sync.Mutex
	queue         matchQueue
	games         map[identity.GameID]*game
	profileGames  map[identity.ProfileID]identity.GameID
	pickColor     func() chess.Color // Always White or Black.
	onGameExpired func(GameSnapshot)
	repository    persistence.GameRepository
	profiles      persistence.ProfileRepository
	notifications sync.WaitGroup
	closed        bool
}

// NewGameService returns an empty gameplay service backed by repository.
// A repository is required.
func NewGameService(repository persistence.GameRepository, profiles persistence.ProfileRepository) *GameService {
	if repository == nil {
		panic("gameplay: GameRepository is required")
	}
	if profiles == nil {
		panic("gameplay: ProfileRepository is required")
	}
	return &GameService{
		repository:   repository,
		profiles:     profiles,
		queue:        make(matchQueue),
		games:        make(map[identity.GameID]*game),
		profileGames: make(map[identity.ProfileID]identity.GameID),
		pickColor: func() chess.Color {
			if rand.IntN(2) == 0 {
				return chess.White
			}
			return chess.Black
		},
	}
}

// ValidateProfile checks the durable player identity before connection admission.
func (s *GameService) ValidateProfile(ctx context.Context, profileID identity.ProfileID) error {
	if err := validateProfileID(profileID); err != nil {
		return err
	}
	_, err := s.profiles.GetProfileByID(ctx, profileID)
	if errors.Is(err, persistence.ErrProfileNotFound) {
		return ErrProfileNotFound
	}
	return err
}

// SetGameExpiredHandler sets the handler called asynchronously when a game clock expires.
// The handler may be called concurrently and must return. Close waits for it.
func (s *GameService) SetGameExpiredHandler(handler func(GameSnapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onGameExpired = handler
}

func (s *GameService) notifyGameExpired(state GameSnapshot) {
	handler := s.onGameExpired
	if handler == nil || s.closed {
		return
	}

	// Notifications may call back into transport code that is waiting on this operation.
	s.notifications.Go(func() {
		handler(state)
	})
}

// ActiveGame returns the unfinished game for a profile, or an empty ID.
func (s *GameService) ActiveGame(profileID identity.ProfileID) identity.GameID {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.activeGame(profileID)
}

func (s *GameService) activeGame(profileID identity.ProfileID) identity.GameID {
	id := s.profileGames[profileID]
	if id != "" && s.games[id].active() {
		return id
	}

	return ""
}

func (s *GameService) validateAdmission(profileID identity.ProfileID) error {
	if s.activeGame(profileID) != "" {
		return ErrAlreadyInGame
	}

	s.queue.removeCanceledPlayers()
	if s.queue.containsProfile(profileID) {
		return ErrAlreadyQueued
	}
	return nil
}

// CreatePrivateGame creates a private game using the requested time control and color preference.
func (s *GameService) CreatePrivateGame(_ context.Context, command CreatePrivateCommand) (GameAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	if err = s.validateAdmission(command.ProfileID); err != nil {
		return GameAssignment{}, err
	}

	s.games[gameID] = game
	s.profileGames[command.ProfileID] = gameID

	slog.Info("private game created", "event", "game.created", "game_id", gameID,
		"profile_id", command.ProfileID, "mode", "private",
		"initial_ms", command.Initial.Milliseconds(), "increment_ms", command.Increment.Milliseconds())
	return GameAssignment{GameID: gameID, Color: creatorColor}, nil
}

// JoinPrivateGame seats a profile in the open position of a private game.
func (s *GameService) JoinPrivateGame(_ context.Context, command JoinPrivateCommand) (GameAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateProfileID(command.ProfileID); err != nil {
		return GameAssignment{}, err
	}

	game, ok := s.games[command.GameID]
	if !ok {
		return GameAssignment{}, ErrGameNotFound
	}

	if s.activeGame(command.ProfileID) == command.GameID {
		return GameAssignment{}, ErrAlreadyParticipant
	}

	if err := s.validateAdmission(command.ProfileID); err != nil {
		return GameAssignment{}, err
	}

	color, err := game.joinPrivate(command)
	if err != nil {
		return GameAssignment{}, err
	}

	s.profileGames[command.ProfileID] = command.GameID
	s.scheduleExpiration(game)
	return GameAssignment{GameID: command.GameID, Color: color}, nil
}

// ResumeGame returns the current state, rejecting profiles that are not participants.
func (s *GameService) ResumeGame(_ context.Context, command ResumeGameCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateProfileID(command.ProfileID); err != nil {
		return GameSnapshot{}, err
	}

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer s.notifyFinished(game)

	state, err := game.resume(command.ProfileID)
	if err == nil {
		slog.Info("game resumed", "event", "game.resumed", "game_id", command.GameID,
			"profile_id", command.ProfileID, "outcome", state.Outcome, "version", state.Version)
	}
	return state, err
}

// EnterMatchmaking queues a profile or matches it with an opponent using the same time control.
// Canceled contexts are ineligible for pairing; callers use CancelMatchmaking
// to release abandoned entries promptly, even if there are no further admissions.
func (s *GameService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchmakingResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return MatchmakingResult{}, err
	}
	if err := validateProfileID(command.ProfileID); err != nil {
		return MatchmakingResult{}, err
	}

	if err := validateTimeControl(command.Initial, command.Increment); err != nil {
		return MatchmakingResult{}, err
	}

	if err := s.validateAdmission(command.ProfileID); err != nil {
		return MatchmakingResult{}, err
	}

	player := &MatchmakingEntry{command: command, ctx: ctx}
	waiting := s.queue.findOpponent(player)
	if waiting == nil {
		slog.Info("player queued", "event", "matchmaking.queued", "profile_id", command.ProfileID,
			"initial_ms", command.Initial.Milliseconds(), "increment_ms", command.Increment.Milliseconds())
		return MatchmakingResult{Entry: player}, nil
	}
	return MatchmakingResult{Entry: player, Opponent: waiting, GameID: s.createMatch(waiting, player)}, nil
}

// CancelMatchmaking removes only this admission if it is still queued.
// It is safe to repeat, pass nil, or cancel after a match has committed.
func (s *GameService) CancelMatchmaking(entry *MatchmakingEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.remove(entry)
}

// createMatch stores the game before returning it to the admission caller.
func (s *GameService) createMatch(waiting, current *MatchmakingEntry) identity.GameID {
	waitingColor := s.pickColor()

	whiteProfileID, blackProfileID := assignColors(
		waiting.command.ProfileID,
		current.command.ProfileID,
		waitingColor,
	)

	gameID := identity.NewGameID()
	game := newGame(gameID, whiteProfileID, blackProfileID, current.command.Initial, current.command.Increment)

	s.games[gameID] = game
	s.profileGames[whiteProfileID] = gameID
	s.profileGames[blackProfileID] = gameID
	s.scheduleExpiration(game)

	slog.Info("game started", "event", "game.started", "game_id", gameID, "mode", "matchmaking",
		"white_profile_id", whiteProfileID, "black_profile_id", blackProfileID,
		"initial_ms", current.command.Initial.Milliseconds(), "increment_ms", current.command.Increment.Milliseconds())

	return gameID
}

// MakeMove applies a move to the identified game on behalf of a profile.
func (s *GameService) MakeMove(_ context.Context, command MoveCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}

	defer s.notifyFinished(game)

	state, err := game.move(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	s.scheduleExpiration(game)
	return state, nil
}

// GameState returns the current snapshot, settling an expired clock before returning.
func (s *GameService) GameState(_ context.Context, gameID identity.GameID) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(gameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	now := game.now()
	game.expire(now)
	state := game.snapshot(now)
	s.notifyFinished(game)
	return state, nil
}

// Resign resigns a game on behalf of a participant.
func (s *GameService) Resign(_ context.Context, command ResignCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer s.notifyFinished(game)

	snapshot, err := game.resign(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	s.stopExpiration(game)
	return snapshot, nil
}

// OfferDraw makes a draw offer on behalf of a game participant.
func (s *GameService) OfferDraw(_ context.Context, command OfferDrawCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer s.notifyFinished(game)

	return game.offerDraw(command)
}

// AcceptDraw accepts a pending draw offer on behalf of a game participant.
func (s *GameService) AcceptDraw(_ context.Context, command DrawOfferResponseCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer s.notifyFinished(game)

	snapshot, err := game.acceptDraw(command)
	if err != nil {
		return GameSnapshot{}, err
	}

	s.stopExpiration(game)
	return snapshot, nil
}

// DeclineDraw declines a pending draw offer on behalf of a game participant.
func (s *GameService) DeclineDraw(_ context.Context, command DrawOfferResponseCommand) (GameSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, err := s.gameByID(command.GameID)
	if err != nil {
		return GameSnapshot{}, err
	}
	defer s.notifyFinished(game)

	return game.declineDraw(command)
}

// Close stops pending expiration timers and waits for notifications and saves.
// Call it after application operations have stopped, never from a notification handler.
func (s *GameService) Close() {
	s.mu.Lock()
	s.closed = true
	clear(s.queue)
	for _, game := range s.games {
		s.stopExpiration(game)
	}
	s.mu.Unlock()
	s.notifications.Wait()
}

func (s *GameService) gameByID(id identity.GameID) (*game, error) {
	game, ok := s.games[id]
	if !ok {
		return nil, ErrGameNotFound
	}
	return game, nil
}

func (s *GameService) scheduleExpiration(game *game) {
	s.stopExpiration(game)
	deadline, running := game.clock.deadline(game.engine.Position().Turn())
	if !running {
		return
	}
	expirationID := game.expirationID
	game.expirationTimer = time.AfterFunc(deadline.Sub(game.now()), func() {
		s.expireFromTimer(game, expirationID)
	})
}

func (s *GameService) expireFromTimer(game *game, expirationID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Replaced timers can already have a callback waiting to acquire mu.
	if s.closed || game.expirationID != expirationID {
		return
	}
	game.expirationTimer = nil
	game.expire(game.now())
	s.notifyFinished(game)
}

func (s *GameService) notifyFinished(game *game) {
	if game.outcome == domain.OutcomeNone {
		return
	}
	if !game.completionScheduled && !s.closed {
		game.completionScheduled = true
		record := completedGameDTO(game.completedGame())
		s.notifications.Go(func() { s.saveCompletedGame(record) })
	}
	if game.termination != domain.TerminationTimeout || game.expirationNotified {
		return
	}
	game.expirationNotified = true
	s.stopExpiration(game)
	s.notifyGameExpired(game.snapshot(game.now()))
}

func (s *GameService) saveCompletedGame(record persistence.CompletedGame) {
	for attempt := 1; attempt <= 3; attempt++ {
		// A disconnected player cannot cancel persistence of a committed result.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := s.repository.CreateGame(ctx, record)
		cancel()
		if err == nil {
			slog.Info("completed game saved", "event", "game.persisted", "game_id", record.ID)
			return
		}
		if attempt == 3 {
			slog.Error("save completed game", "event", "game.persistence_failed", "game_id", record.ID,
				"attempts", attempt, "error", err)
			return
		}
		slog.Warn("retry completed game save", "event", "game.persistence_retry", "game_id", record.ID,
			"attempt", attempt, "error", err)
		time.Sleep(time.Second)
	}
}

func (s *GameService) stopExpiration(game *game) {
	if game.expirationTimer != nil {
		game.expirationTimer.Stop()
		game.expirationTimer = nil
	}
	game.expirationID++
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
