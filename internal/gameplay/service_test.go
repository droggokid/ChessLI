package gameplay

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
	"ChessLI/internal/persistence"

	"github.com/corentings/chess/v2"
	"go.uber.org/mock/gomock"
)

func newTestGameService(t *testing.T) *GameService {
	t.Helper()
	repository := persistence.NewMockGameRepository(gomock.NewController(t))
	repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	service := NewGameService(repository, newTestProfileRepository(t))
	t.Cleanup(service.Close)
	return service
}

func newTestProfileRepository(t *testing.T) *persistence.MockProfileRepository {
	t.Helper()
	repository := persistence.NewMockProfileRepository(gomock.NewController(t))
	repository.EXPECT().GetProfileByID(gomock.Any(), gomock.Any()).Return(persistence.Profile{}, nil).AnyTimes()
	return repository
}

func TestGameServiceRejectsUnknownProfile(t *testing.T) {
	profiles := persistence.NewMockProfileRepository(gomock.NewController(t))
	id := identity.NewProfileID()
	profiles.EXPECT().GetProfileByID(gomock.Any(), id).Return(persistence.Profile{}, persistence.ErrProfileNotFound)
	repository := persistence.NewMockGameRepository(gomock.NewController(t))
	service := NewGameService(repository, profiles)
	defer service.Close()
	if err := service.ValidateProfile(t.Context(), id); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("ValidateProfile() = %v, want profile not found", err)
	}
}

func TestGameServicePrivateGameLifecycle(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	creator := identity.ProfileID("creator")
	joiner := identity.ProfileID("joiner")

	created, err := service.CreatePrivateGame(context.Background(), CreatePrivateCommand{
		ProfileID:       creator,
		Initial:         10 * time.Minute,
		Increment:       5 * time.Second,
		ColorPreference: ColorWhite,
	})
	if err != nil {
		t.Fatalf("CreatePrivateGame() error = %v", err)
	}
	if created.GameID == "" {
		t.Fatal("CreatePrivateGame() returned an empty game ID")
	}
	if created.Color != chess.White {
		t.Fatalf("CreatePrivateGame() color = %v, want %v", created.Color, chess.White)
	}

	game, err := service.gameByID(created.GameID)
	if err != nil {
		t.Fatalf("gameByID() error = %v", err)
	}
	fixedNow := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	game.now = func() time.Time { return fixedNow }

	joined, err := service.JoinPrivateGame(context.Background(), JoinPrivateCommand{
		ProfileID: joiner,
		GameID:    created.GameID,
	})
	if err != nil {
		t.Fatalf("JoinPrivateGame() error = %v", err)
	}
	if joined.Color != chess.Black {
		t.Fatalf("JoinPrivateGame() color = %v, want %v", joined.Color, chess.Black)
	}

	state, err := service.GameState(context.Background(), created.GameID)
	if err != nil {
		t.Fatalf("GameState() error = %v", err)
	}
	if state.WhiteProfileID != creator || state.BlackProfileID != joiner {
		t.Fatalf("GameState() players = (%q, %q), want (%q, %q)", state.WhiteProfileID, state.BlackProfileID, creator, joiner)
	}
	if state.WhiteRemaining != 10*time.Minute || state.BlackRemaining != 10*time.Minute {
		t.Fatalf("GameState() remaining = (%v, %v), want 10m for both", state.WhiteRemaining, state.BlackRemaining)
	}
}

func TestGameServiceConcurrentSnapshotsAndMoves(t *testing.T) {
	service := newTestGameService(t)
	defer service.Close()
	ctx := context.Background()
	created, err := service.CreatePrivateGame(ctx, CreatePrivateCommand{
		ProfileID: "white", Initial: time.Minute, ColorPreference: ColorWhite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.JoinPrivateGame(ctx, JoinPrivateCommand{GameID: created.GameID, ProfileID: "black"}); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	var readers sync.WaitGroup
	defer readers.Wait()
	for range 2 {
		readers.Go(func() {
			<-start
			for range 8 {
				state, err := service.GameState(ctx, created.GameID)
				if err != nil {
					t.Error(err)
					return
				}
				if state.GameID != created.GameID || state.FEN == "" {
					t.Errorf("incomplete snapshot: %+v", state)
				}
			}
		})
	}
	close(start)
	for i, move := range []string{"e2e4", "e7e5", "g1f3", "b8c6"} {
		player := identity.ProfileID("white")
		if i%2 != 0 {
			player = "black"
		}
		state, err := service.MakeMove(ctx, MoveCommand{
			GameID: created.GameID, ProfileID: player, Move: move, ExpectedVersion: uint64(i),
		})
		if err != nil {
			t.Fatal(err)
		}
		if state.Version != uint64(i+1) {
			t.Fatalf("move %s returned version %d", move, state.Version)
		}
	}
}

func TestGameServiceResumeGame(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	game := newReadyGame()
	service.games[game.id] = game

	for _, profileID := range []identity.ProfileID{"white", "black"} {
		state, err := service.ResumeGame(context.Background(), ResumeGameCommand{
			GameID:    game.id,
			ProfileID: profileID,
		})
		if err != nil {
			t.Fatalf("ResumeGame(%q) error = %v", profileID, err)
		}
		if state.GameID != game.id {
			t.Fatalf("ResumeGame(%q) game ID = %q, want %q", profileID, state.GameID, game.id)
		}
	}

	_, err := service.ResumeGame(context.Background(), ResumeGameCommand{
		GameID:    game.id,
		ProfileID: "spectator",
	})
	if !errors.Is(err, ErrNotParticipant) {
		t.Fatalf("ResumeGame() spectator error = %v, want %v", err, ErrNotParticipant)
	}
}

func TestGameServiceAutomaticallyExpiresPrivateGame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := newTestGameService(t)
		defer service.Close()

		expired := make(chan GameSnapshot, 1)
		service.SetGameExpiredHandler(func(state GameSnapshot) {
			expired <- state
		})

		created, err := service.CreatePrivateGame(t.Context(), CreatePrivateCommand{
			ProfileID:       "white",
			Initial:         20 * time.Millisecond,
			ColorPreference: ColorWhite,
		})
		if err != nil {
			t.Fatalf("CreatePrivateGame() error = %v", err)
		}

		if _, err = service.JoinPrivateGame(t.Context(), JoinPrivateCommand{
			GameID:    created.GameID,
			ProfileID: "black",
		}); err != nil {
			t.Fatalf("JoinPrivateGame() error = %v", err)
		}

		select {
		case state := <-expired:
			if state.Outcome != domain.OutcomeBlackWin || state.Termination != domain.TerminationTimeout {
				t.Fatalf("expired state = (%v, %v), want (%v, %v)", state.Outcome, state.Termination, domain.OutcomeBlackWin, domain.TerminationTimeout)
			}
			if state.Version != 1 || state.WhiteRemaining != 0 {
				t.Fatalf("expired state version/time = (%d, %v), want (1, 0)", state.Version, state.WhiteRemaining)
			}
		case <-time.After(time.Second):
			t.Fatal("game did not expire automatically")
		}
	})
}

func TestGameServiceNotifiesTimeoutDetectedByCommandsOrState(t *testing.T) {
	tests := []struct {
		name string
		act  func(*GameService, identity.GameID) error
	}{
		{name: "move", act: func(s *GameService, id identity.GameID) error {
			_, err := s.MakeMove(context.Background(), MoveCommand{GameID: id, ProfileID: "white", Move: "e2e4", Notation: MoveNotationUCI})
			return err
		}},
		{name: "resign", act: func(s *GameService, id identity.GameID) error {
			_, err := s.Resign(context.Background(), ResignCommand{GameID: id, ProfileID: "white"})
			return err
		}},
		{name: "offer draw", act: func(s *GameService, id identity.GameID) error {
			_, err := s.OfferDraw(context.Background(), OfferDrawCommand{GameID: id, ProfileID: "white"})
			return err
		}},
		{name: "accept draw", act: func(s *GameService, id identity.GameID) error {
			_, err := s.AcceptDraw(context.Background(), DrawOfferResponseCommand{GameID: id, ProfileID: "black", OfferID: "offer"})
			return err
		}},
		{name: "decline draw", act: func(s *GameService, id identity.GameID) error {
			_, err := s.DeclineDraw(context.Background(), DrawOfferResponseCommand{GameID: id, ProfileID: "black", OfferID: "offer"})
			return err
		}},
		{name: "state", act: func(s *GameService, id identity.GameID) error {
			_, err := s.GameState(context.Background(), id)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestGameService(t)
			game := newReadyGame()
			now := time.Now()
			game.now = func() time.Time { return now }
			game.clock = newGameClock(time.Second, 0)
			game.clock.start(now)
			service.games[game.id] = game
			now = now.Add(time.Second)

			expired := make(chan GameSnapshot, 1)
			service.SetGameExpiredHandler(func(state GameSnapshot) { expired <- state })
			_ = tt.act(service, game.id)

			select {
			case state := <-expired:
				if state.Termination != domain.TerminationTimeout {
					t.Fatalf("expiration termination = %v, want timeout", state.Termination)
				}
			case <-time.After(time.Second):
				t.Fatal("timeout did not notify")
			}
			_ = tt.act(service, game.id)
			service.Close()
			select {
			case state := <-expired:
				t.Fatalf("duplicate timeout notification = %+v", state)
			default:
			}
		})
	}
}

func TestGameServiceCreatePrivateGameRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command CreatePrivateCommand
		wantErr error
	}{
		{
			name:    "invalid time control",
			command: CreatePrivateCommand{ProfileID: "player", Initial: 0, ColorPreference: ColorWhite},
			wantErr: ErrInvalidTimeControl,
		},
		{
			name:    "invalid color",
			command: CreatePrivateCommand{ProfileID: "player", Initial: time.Minute, ColorPreference: ColorPreference(99)},
			wantErr: ErrInvalidColorPreference,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := newTestGameService(t).CreatePrivateGame(context.Background(), tt.command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreatePrivateGame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGameServiceRejectsEmptyProfileID(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	created, err := service.CreatePrivateGame(context.Background(), CreatePrivateCommand{
		Initial: time.Minute, ColorPreference: ColorWhite,
	})
	if !errors.Is(err, ErrInvalidProfileID) {
		t.Fatalf("CreatePrivateGame() error = %v, want %v", err, ErrInvalidProfileID)
	}

	created, err = service.CreatePrivateGame(context.Background(), CreatePrivateCommand{
		ProfileID: "white", Initial: time.Minute, ColorPreference: ColorWhite,
	})
	if err != nil {
		t.Fatalf("CreatePrivateGame() error = %v", err)
	}
	if _, err = service.JoinPrivateGame(context.Background(), JoinPrivateCommand{GameID: created.GameID}); !errors.Is(err, ErrInvalidProfileID) {
		t.Fatalf("JoinPrivateGame() error = %v, want %v", err, ErrInvalidProfileID)
	}
	if _, err = service.EnterMatchmaking(context.Background(), EnterMatchmakingCommand{ProfileID: "", Initial: time.Minute, Increment: 0}); !errors.Is(err, ErrInvalidProfileID) {
		t.Fatalf("EnterMatchmaking() error = %v, want %v", err, ErrInvalidProfileID)
	}
}

func TestGameServiceMatchmakingMatchesWithinPool(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := newTestGameService(t)
		service.pickColor = func() chess.Color { return chess.White }
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		first, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "first", Initial: 3 * time.Minute, Increment: 2 * time.Second})
		if err != nil {
			t.Fatalf("first EnterMatchmaking() error = %v", err)
		}
		if first.Entry == nil || first.Opponent != nil || first.GameID != "" {
			t.Fatalf("first admission = %+v, want queued entry", first)
		}

		second, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "second", Initial: 3 * time.Minute, Increment: 2 * time.Second})
		if err != nil {
			t.Fatalf("second EnterMatchmaking() error = %v", err)
		}

		if second.GameID == "" || second.Opponent != first.Entry || second.Entry == first.Entry {
			t.Fatalf("second admission = %+v, want match with first entry", second)
		}

		cancel()
		service.CancelMatchmaking(first.Entry)
		service.CancelMatchmaking(second.Entry)
		for _, profileID := range []identity.ProfileID{"first", "second"} {
			if service.ActiveGame(profileID) != second.GameID {
				t.Fatalf("cancellation released matched player %q", profileID)
			}
		}
		state, err := service.GameState(t.Context(), second.GameID)
		if err != nil || state.Outcome != domain.OutcomeNone {
			t.Fatalf("cancellation changed matched game: %+v, %v", state, err)
		}
		if state.WhiteProfileID != "first" || state.BlackProfileID != "second" {
			t.Fatalf("match players = (%q, %q), want (first, second)", state.WhiteProfileID, state.BlackProfileID)
		}
	})
}

func TestGameServiceMatchmakingKeepsPoolsSeparate(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bullet, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "bullet-1", Initial: time.Minute, Increment: 0})
	if err != nil {
		t.Fatalf("bullet EnterMatchmaking() error = %v", err)
	}
	rapid, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "rapid-1", Initial: 10 * time.Minute, Increment: 0})
	if err != nil {
		t.Fatalf("rapid EnterMatchmaking() error = %v", err)
	}
	if bullet.GameID != "" || rapid.GameID != "" {
		t.Fatal("different pools matched")
	}

	bulletPeer, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "bullet-2", Initial: time.Minute, Increment: 0})
	if err != nil {
		t.Fatalf("bullet peer EnterMatchmaking() error = %v", err)
	}
	rapidPeer, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "rapid-2", Initial: 10 * time.Minute, Increment: 0})
	if err != nil {
		t.Fatalf("rapid peer EnterMatchmaking() error = %v", err)
	}

	if bulletPeer.GameID == "" || bulletPeer.Opponent != bullet.Entry {
		t.Fatalf("bullet match = %+v, want original bullet entry", bulletPeer)
	}
	if rapidPeer.GameID == "" || rapidPeer.Opponent != rapid.Entry {
		t.Fatalf("rapid match = %+v, want original rapid entry", rapidPeer)
	}
	if bulletPeer.GameID == rapidPeer.GameID {
		t.Fatalf("different pools created the same game %q", bulletPeer.GameID)
	}
}

func TestGameServiceMatchmakingRejectsDuplicateProfileAcrossPools(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "player", Initial: time.Minute, Increment: 0})
	if err != nil {
		t.Fatalf("first EnterMatchmaking() error = %v", err)
	}
	_, err = service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "player", Initial: 10 * time.Minute, Increment: 0})
	if !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("second EnterMatchmaking() error = %v, want %v", err, ErrAlreadyQueued)
	}
}

func TestGameServiceCancelMatchmakingRemovesOnlyItsEntry(t *testing.T) {
	t.Parallel()

	service := newTestGameService(t)
	ctx, cancel := context.WithCancel(context.Background())
	command := EnterMatchmakingCommand{ProfileID: "player", Initial: time.Minute}
	first, err := service.EnterMatchmaking(ctx, command)
	if err != nil {
		t.Fatalf("EnterMatchmaking() error = %v", err)
	}

	cancel()
	service.CancelMatchmaking(first.Entry)
	if len(service.queue) != 0 {
		t.Fatal("explicit cancellation left a queued entry")
	}
	second, err := service.EnterMatchmaking(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	service.CancelMatchmaking(first.Entry)
	service.CancelMatchmaking(nil)
	if len(service.queue) != 1 || second.Entry == first.Entry {
		t.Fatal("late cancellation removed the replacement entry")
	}
	service.CancelMatchmaking(second.Entry)
	if len(service.queue) != 0 {
		t.Fatal("replacement could not be canceled")
	}
}

func TestGameServiceMatchmakingSkipsCanceledEntries(t *testing.T) {
	service := newTestGameService(t)
	ctx, cancel := context.WithCancel(t.Context())
	first, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "old", Initial: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	second, err := service.EnterMatchmaking(t.Context(), EnterMatchmakingCommand{ProfileID: "new", Initial: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if second.GameID != "" || second.Opponent != nil || service.ActiveGame("old") != "" {
		t.Fatal("paired with a canceled entry")
	}
	service.CancelMatchmaking(first.Entry)
	third, err := service.EnterMatchmaking(t.Context(), EnterMatchmakingCommand{ProfileID: "old", Initial: time.Minute})
	if err != nil || third.GameID == "" || third.Opponent != second.Entry {
		t.Fatalf("late cancellation affected replacement or re-admission: %+v, %v", third, err)
	}
}

func TestGameServiceMatchmakingRejectsInvalidTimeControl(t *testing.T) {
	t.Parallel()

	_, err := newTestGameService(t).EnterMatchmaking(
		context.Background(),
		EnterMatchmakingCommand{ProfileID: "player", Initial: 0, Increment: 0},
	)
	if !errors.Is(err, ErrInvalidTimeControl) {
		t.Fatalf("EnterMatchmaking() error = %v, want %v", err, ErrInvalidTimeControl)
	}
}

func TestGameServiceCancellationOnlyStopsQueuedWork(t *testing.T) {
	t.Parallel()
	service := newTestGameService(t)
	defer service.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := service.EnterMatchmaking(ctx, EnterMatchmakingCommand{
		ProfileID: "white", Initial: time.Minute,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled matchmaking error = %v, want context.Canceled", err)
	}

	created, err := service.CreatePrivateGame(ctx, CreatePrivateCommand{
		ProfileID: "white", Initial: time.Minute, ColorPreference: ColorWhite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.JoinPrivateGame(ctx, JoinPrivateCommand{
		ProfileID: "black", GameID: created.GameID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MakeMove(ctx, MoveCommand{
		ProfileID: "white", GameID: created.GameID, Move: "e2e4",
	}); err != nil {
		t.Fatal(err)
	}
	state, err := service.GameState(ctx, created.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != 1 || state.LastMoveSAN != "e4" {
		t.Fatalf("state after canceled synchronous calls = %+v, want committed e4", state)
	}
}

func TestGameServiceConcurrentCreateReservesProfileOnce(t *testing.T) {
	s := newTestGameService(t)
	defer s.Close()
	start := make(chan struct{})
	results := make(chan error, 16)
	for range cap(results) {
		go func() {
			<-start
			_, err := s.CreatePrivateGame(context.Background(), CreatePrivateCommand{
				ProfileID: "player", Initial: time.Minute, ColorPreference: ColorWhite,
			})
			results <- err
		}()
	}
	close(start)
	successes := 0
	for range cap(results) {
		if err := <-results; err == nil {
			successes++
		} else if !errors.Is(err, ErrAlreadyInGame) {
			t.Fatalf("CreatePrivateGame() error = %v", err)
		}
	}
	if successes != 1 || len(s.games) != 1 || s.ActiveGame("player") == "" {
		t.Fatalf("successes/games/active = %d/%d/%q", successes, len(s.games), s.ActiveGame("player"))
	}
}

func TestGameServiceAdmissionAcrossModes(t *testing.T) {
	s := newTestGameService(t)
	defer s.Close()
	ctx := context.Background()
	create := func(profile identity.ProfileID) (GameAssignment, error) {
		return s.CreatePrivateGame(ctx, CreatePrivateCommand{
			ProfileID: profile, Initial: time.Minute, ColorPreference: ColorWhite,
		})
	}
	host, err := create("host")
	if err != nil {
		t.Fatal(err)
	}
	queued, cancel := context.WithCancel(ctx)
	defer cancel()
	if _, err = s.EnterMatchmaking(queued, EnterMatchmakingCommand{ProfileID: "player", Initial: time.Minute}); err != nil {
		t.Fatal(err)
	}
	if _, err = create("player"); !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("queued create error = %v", err)
	}
	if _, err = s.JoinPrivateGame(ctx, JoinPrivateCommand{GameID: host.GameID, ProfileID: "player"}); !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("queued join error = %v", err)
	}
	cancel()
	created, err := create("player")
	if err != nil {
		t.Fatalf("create after cancellation: %v", err)
	}
	if s.ActiveGame("player") != created.GameID {
		t.Fatal("private creator has no active game")
	}
	if _, err = s.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "player", Initial: time.Minute}); !errors.Is(err, ErrAlreadyInGame) {
		t.Fatalf("private to matchmaking error = %v", err)
	}
	if _, err = s.JoinPrivateGame(ctx, JoinPrivateCommand{GameID: host.GameID, ProfileID: "player"}); !errors.Is(err, ErrAlreadyInGame) {
		t.Fatalf("active join error = %v", err)
	}
	if _, err = s.JoinPrivateGame(ctx, JoinPrivateCommand{GameID: host.GameID, ProfileID: "joiner"}); err != nil {
		t.Fatal(err)
	}
	if s.ActiveGame("joiner") != host.GameID {
		t.Fatal("private joiner has no active game")
	}
	first, err := s.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "first", Initial: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.EnterMatchmaking(ctx, EnterMatchmakingCommand{ProfileID: "second", Initial: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if second.Opponent != first.Entry || second.GameID == "" || s.ActiveGame("first") != second.GameID || s.ActiveGame("second") != second.GameID {
		t.Fatal("matchmaking did not reserve both profiles")
	}
	if _, err = create("first"); !errors.Is(err, ErrAlreadyInGame) {
		t.Fatalf("matchmaking to private error = %v", err)
	}
}

func TestGameServiceActiveGameReleasesFinishedOrExpired(t *testing.T) {
	for _, name := range []string{"resigned", "expired"} {
		t.Run(name, func(t *testing.T) {
			s := newTestGameService(t)
			defer s.Close()
			g := newReadyGame()
			s.games[g.id] = g
			s.profileGames["white"], s.profileGames["black"] = g.id, g.id
			if name == "expired" {
				g.now = func() time.Time { return g.clock.turnStartedAt.Add(10 * time.Minute) }
			} else if _, err := s.Resign(context.Background(), ResignCommand{GameID: g.id, ProfileID: "white"}); err != nil {
				t.Fatal(err)
			}
			if s.ActiveGame("white") != "" || s.ActiveGame("black") != "" {
				t.Fatal("finished or elapsed game still reserves profiles")
			}
			if _, err := s.CreatePrivateGame(context.Background(), CreatePrivateCommand{
				ProfileID: "white", Initial: time.Minute, ColorPreference: ColorWhite,
			}); err != nil {
				t.Fatalf("create after finish or expiry: %v", err)
			}
		})
	}
}

func TestGameServiceCloseJoinsAsynchronousNotifications(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := newTestGameService(t)
		notified := make(chan GameSnapshot, 2)
		release := make(chan struct{})
		service.SetGameExpiredHandler(func(state GameSnapshot) {
			notified <- state
			<-release
		})
		service.mu.Lock()
		service.notifyGameExpired(GameSnapshot{GameID: "game"})
		service.mu.Unlock()
		if state := <-notified; state.GameID != "game" {
			t.Fatalf("notification = %+v", state)
		}
		closed := make(chan struct{})
		go func() {
			service.Close()
			close(closed)
		}()
		synctest.Wait()
		select {
		case <-closed:
			t.Error("Close returned while a notification handler was still running")
		default:
		}
		close(release)
		<-closed
		service.mu.Lock()
		service.notifyGameExpired(GameSnapshot{GameID: "after-close"})
		service.mu.Unlock()
		synctest.Wait()
		if len(notified) != 0 {
			t.Fatal("notification started after Close")
		}
	})
}

func TestGameServiceCompletionCapturesMovesAndFinalClocks(t *testing.T) {
	repository := persistence.NewMockGameRepository(gomock.NewController(t))
	service := NewGameService(repository, newTestProfileRepository(t))
	defer service.Close()
	g := newGame("game", "white", "black", time.Minute, time.Second)
	now := time.Now()
	g.now = func() time.Time { return now }
	g.clock.start(now)
	service.games[g.id] = g
	completed := make(chan persistence.CompletedGame, 2)
	repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, record persistence.CompletedGame) error {
		// Reentry proves database work runs outside the gameplay lock.
		if _, err := service.GameState(t.Context(), record.ID); err != nil {
			t.Error(err)
		}
		completed <- record
		return nil
	})
	now = now.Add(2 * time.Second)
	if _, err := service.MakeMove(t.Context(), MoveCommand{
		GameID: g.id, ProfileID: "white", Move: "e4", Notation: MoveNotationSAN,
	}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Second)
	state, err := service.Resign(t.Context(), ResignCommand{GameID: g.id, ProfileID: "black"})
	if err != nil {
		t.Fatal(err)
	}
	service.Close()
	record := <-completed
	if record.ID != g.id || record.InitialTimeMS != 60000 || record.IncrementMS != 1000 ||
		record.FinalFEN != state.FEN || !record.CompletedAt.Equal(now) ||
		record.WhiteRemainingMS != 59000 || record.BlackRemainingMS != 57000 {
		t.Fatalf("completed record = %+v", record)
	}
	if len(record.Moves) != 1 || record.Moves[0].UCI != "e2e4" || record.Moves[0].Ply != 1 ||
		record.Moves[0].BlackRemainingMS != 60000 {
		t.Fatalf("recorded moves = %+v", record.Moves)
	}
	record.Moves[0].UCI = "changed"
	if g.moves[0].UCI != "e2e4" || len(completed) != 0 {
		t.Fatal("completion was not detached or was notified more than once")
	}
}

func TestGameServiceCompletionIncludesAutomaticTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repository := persistence.NewMockGameRepository(gomock.NewController(t))
		service := NewGameService(repository, newTestProfileRepository(t))
		defer service.Close()
		completed := make(chan persistence.CompletedGame, 1)
		repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, record persistence.CompletedGame) error {
			completed <- record
			return nil
		})
		assignment, err := service.CreatePrivateGame(t.Context(), CreatePrivateCommand{
			ProfileID: "white", Initial: time.Second, ColorPreference: ColorWhite,
		})
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		if _, err := service.JoinPrivateGame(t.Context(), JoinPrivateCommand{
			GameID: assignment.GameID, ProfileID: "black",
		}); err != nil {
			t.Fatal(err)
		}
		record := <-completed
		if record.Termination != domain.TerminationTimeout || record.WhiteRemainingMS != 0 ||
			!record.CompletedAt.Equal(started.Add(time.Second)) || len(record.Moves) != 0 {
			t.Fatalf("timeout record = %+v", record)
		}
	})
}

func TestGameServiceRetriesSaveWithoutBlockingFinalState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repository := persistence.NewMockGameRepository(gomock.NewController(t))
		started := make(chan struct{})
		first := repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ persistence.CompletedGame) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
		repository.EXPECT().CreateGame(gomock.Any(), gomock.Any()).Return(nil).After(first)
		service := NewGameService(repository, newTestProfileRepository(t))
		defer service.Close()
		g := newReadyGame()
		service.games[g.id] = g
		state, err := service.Resign(t.Context(), ResignCommand{GameID: g.id, ProfileID: "white"})
		if err != nil {
			t.Fatal(err)
		}
		<-started
		if state.Outcome != domain.OutcomeBlackWin {
			t.Fatalf("resignation outcome = %s", state.Outcome)
		}
		if _, err := service.GameState(t.Context(), g.id); err != nil {
			t.Fatal(err)
		}
		// Close waits through the timed-out write and the successful retry.
		service.Close()
	})
}

func TestGameServiceIgnoresStaleTimerAfterMoveFinishOrClose(t *testing.T) {
	for _, action := range []string{"move", "resign", "accept draw", "close"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				service := newTestGameService(t)
				defer service.Close()
				g := newReadyGame()
				now := time.Now()
				g.now = func() time.Time { return now }
				g.clock = newGameClock(time.Minute, 0)
				g.clock.start(now)
				service.games[g.id] = g
				notified := false
				service.SetGameExpiredHandler(func(GameSnapshot) { notified = true })
				service.mu.Lock()
				service.scheduleExpiration(g)
				id := g.expirationID
				deadline, _ := g.clock.deadline(g.engine.Position().Turn())
				service.mu.Unlock()
				switch action {
				case "move":
					if _, err := service.MakeMove(context.Background(), MoveCommand{GameID: g.id, ProfileID: "white", Move: "e2e4"}); err != nil {
						t.Fatal(err)
					}
				case "resign":
					if _, err := service.Resign(context.Background(), ResignCommand{GameID: g.id, ProfileID: "white"}); err != nil {
						t.Fatal(err)
					}
				case "accept draw":
					state, err := service.OfferDraw(t.Context(), OfferDrawCommand{GameID: g.id, ProfileID: "white"})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := service.AcceptDraw(t.Context(), DrawOfferResponseCommand{GameID: g.id, ProfileID: "black", OfferID: state.PendingDrawOffer.OfferID}); err != nil {
						t.Fatal(err)
					}
				case "close":
					service.Close()
				}
				service.mu.Lock()
				retainedTimer := g.expirationTimer != nil
				now = deadline
				service.mu.Unlock()
				if action != "move" && retainedTimer {
					t.Fatal("finished or closed game retained its expiration timer")
				}
				service.expireFromTimer(g, id)
				synctest.Wait()
				service.mu.Lock()
				outcome := g.outcome
				service.mu.Unlock()
				want := domain.OutcomeNone
				if action == "resign" {
					want = domain.OutcomeBlackWin
				}
				if action == "accept draw" {
					want = domain.OutcomeDraw
				}
				if outcome != want || notified {
					t.Fatal("stale timer expired the game")
				}
			})
		})
	}
}
