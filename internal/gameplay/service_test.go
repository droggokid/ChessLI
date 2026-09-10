package gameplay

import (
	"context"
	"errors"
	"testing"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

func TestValidateTimeControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		initial   time.Duration
		increment time.Duration
		wantErr   bool
	}{
		{name: "valid", initial: time.Minute, increment: time.Second},
		{name: "zero initial", initial: 0, wantErr: true},
		{name: "negative initial", initial: -time.Second, wantErr: true},
		{name: "negative increment", initial: time.Minute, increment: -time.Second, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTimeControl(tt.initial, tt.increment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTimeControl() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGameServicePrivateGameLifecycle(t *testing.T) {
	t.Parallel()

	service := NewGameService()
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

func TestGameServiceAutomaticallyExpiresPrivateGame(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	t.Cleanup(service.Close)

	expired := make(chan GameSnapshot, 1)
	service.SetGameExpiredHandler(func(state GameSnapshot) {
		expired <- state
	})

	created, err := service.CreatePrivateGame(context.Background(), CreatePrivateCommand{
		ProfileID:       "white",
		Initial:         20 * time.Millisecond,
		ColorPreference: ColorWhite,
	})
	if err != nil {
		t.Fatalf("CreatePrivateGame() error = %v", err)
	}

	if _, err = service.JoinPrivateGame(context.Background(), JoinPrivateCommand{
		GameID:    created.GameID,
		ProfileID: "black",
	}); err != nil {
		t.Fatalf("JoinPrivateGame() error = %v", err)
	}

	select {
	case state := <-expired:
		if state.Outcome != chess.BlackWon || state.Termination != TerminationTimeout {
			t.Fatalf("expired state = (%v, %v), want (%v, %v)", state.Outcome, state.Termination, chess.BlackWon, TerminationTimeout)
		}
		if state.Version != 1 || state.WhiteRemaining != 0 {
			t.Fatalf("expired state version/time = (%d, %v), want (1, 0)", state.Version, state.WhiteRemaining)
		}
	case <-time.After(time.Second):
		t.Fatal("game did not expire automatically")
	}
}

func TestGameServiceTerminalActionsStopExpirationTimer(t *testing.T) {
	tests := []struct {
		name string
		act  func(*GameService, *Game) error
	}{
		{
			name: "resign",
			act: func(service *GameService, game *Game) error {
				_, err := service.Resign(context.Background(), NewResignCommand(game.ID, "white"))
				return err
			},
		},
		{
			name: "accept draw",
			act: func(service *GameService, game *Game) error {
				state, err := service.OfferDraw(context.Background(), NewOfferDrawCommand(game.ID, "white"))
				if err != nil {
					return err
				}
				_, err = service.AcceptDraw(context.Background(), NewDrawOfferResponseCommand(game.ID, "black", state.PendingDrawOffer.OfferID))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewGameService()
			game := newReadyGame()
			service.games[game.ID] = game
			game.scheduleExpiration(nil)

			if err := tt.act(service, game); err != nil {
				t.Fatalf("terminal action error = %v", err)
			}

			game.mu.Lock()
			timer := game.expirationTimer
			game.mu.Unlock()
			if timer != nil {
				t.Fatal("terminal action left expiration timer running")
			}
		})
	}
}

func TestGameServiceCreatePrivateGameRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     func() context.Context
		command CreatePrivateCommand
		wantErr error
	}{
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			command: CreatePrivateCommand{Initial: time.Minute, ColorPreference: ColorWhite},
			wantErr: context.Canceled,
		},
		{
			name:    "invalid time control",
			ctx:     context.Background,
			command: CreatePrivateCommand{Initial: 0, ColorPreference: ColorWhite},
			wantErr: ErrInvalidTimeControl,
		},
		{
			name:    "invalid color",
			ctx:     context.Background,
			command: CreatePrivateCommand{Initial: time.Minute, ColorPreference: ColorPreference(99)},
			wantErr: ErrInvalidColorPreference,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewGameService().CreatePrivateGame(tt.ctx(), tt.command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreatePrivateGame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGameServiceMatchmakingMatchesWithinPool(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	service.pickColor = func() chess.Color { return chess.White }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("first", 3*time.Minute, 2*time.Second))
	if err != nil {
		t.Fatalf("first EnterMatchmaking() error = %v", err)
	}
	assertNoMatch(t, first.Result)

	second, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("second", 3*time.Minute, 2*time.Second))
	if err != nil {
		t.Fatalf("second EnterMatchmaking() error = %v", err)
	}

	firstResult := receiveMatch(t, first.Result)
	secondResult := receiveMatch(t, second.Result)
	if firstResult.GameID == "" || firstResult.GameID != secondResult.GameID {
		t.Fatalf("match game IDs = (%q, %q), want same non-empty ID", firstResult.GameID, secondResult.GameID)
	}
	if firstResult.Color != chess.White || secondResult.Color != chess.Black {
		t.Fatalf("match colors = (%v, %v), want (%v, %v)", firstResult.Color, secondResult.Color, chess.White, chess.Black)
	}
}

func TestGameServiceMatchmakingStopsCancellationCleanupAfterMatch(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("first", time.Minute, 0))
	if err != nil {
		t.Fatalf("first EnterMatchmaking() error = %v", err)
	}

	key := timeControlKey{initial: time.Minute}
	service.mu.RLock()
	waiting := service.waitingPlayers[key]
	service.mu.RUnlock()
	if waiting == nil || waiting.stopCleanup == nil {
		t.Fatal("queued player has no cancellation cleanup")
	}

	second, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("second", time.Minute, 0))
	if err != nil {
		t.Fatalf("second EnterMatchmaking() error = %v", err)
	}
	receiveMatch(t, first.Result)
	receiveMatch(t, second.Result)

	service.mu.RLock()
	cleanup := waiting.stopCleanup
	service.mu.RUnlock()
	if cleanup != nil {
		t.Fatal("matched player still has a cancellation cleanup")
	}
}

func TestGameServiceMatchmakingKeepsPoolsSeparate(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bullet, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("bullet-1", time.Minute, 0))
	if err != nil {
		t.Fatalf("bullet EnterMatchmaking() error = %v", err)
	}
	rapid, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("rapid-1", 10*time.Minute, 0))
	if err != nil {
		t.Fatalf("rapid EnterMatchmaking() error = %v", err)
	}
	assertNoMatch(t, bullet.Result)
	assertNoMatch(t, rapid.Result)

	bulletPeer, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("bullet-2", time.Minute, 0))
	if err != nil {
		t.Fatalf("bullet peer EnterMatchmaking() error = %v", err)
	}
	rapidPeer, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("rapid-2", 10*time.Minute, 0))
	if err != nil {
		t.Fatalf("rapid peer EnterMatchmaking() error = %v", err)
	}

	bulletResult := receiveMatch(t, bullet.Result)
	bulletPeerResult := receiveMatch(t, bulletPeer.Result)
	rapidResult := receiveMatch(t, rapid.Result)
	rapidPeerResult := receiveMatch(t, rapidPeer.Result)

	if bulletResult.GameID != bulletPeerResult.GameID {
		t.Fatalf("bullet game IDs = (%q, %q), want same ID", bulletResult.GameID, bulletPeerResult.GameID)
	}
	if rapidResult.GameID != rapidPeerResult.GameID {
		t.Fatalf("rapid game IDs = (%q, %q), want same ID", rapidResult.GameID, rapidPeerResult.GameID)
	}
	if bulletResult.GameID == rapidResult.GameID {
		t.Fatalf("different pools created the same game %q", bulletResult.GameID)
	}
}

func TestGameServiceMatchmakingRejectsDuplicateProfileAcrossPools(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("player", time.Minute, 0))
	if err != nil {
		t.Fatalf("first EnterMatchmaking() error = %v", err)
	}
	_, err = service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("player", 10*time.Minute, 0))
	if !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("second EnterMatchmaking() error = %v, want %v", err, ErrAlreadyQueued)
	}
}

func TestGameServiceMatchmakingRemovesCanceledPlayer(t *testing.T) {
	t.Parallel()

	service := NewGameService()
	ctx, cancel := context.WithCancel(context.Background())
	ticket, err := service.EnterMatchmaking(ctx, NewEnterMatchmakingCommand("player", time.Minute, 0))
	if err != nil {
		t.Fatalf("EnterMatchmaking() error = %v", err)
	}

	cancel()

	select {
	case _, ok := <-ticket.Result:
		if ok {
			t.Fatal("canceled matchmaking ticket unexpectedly produced a match")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled matchmaking ticket was not closed")
	}

	service.mu.RLock()
	remaining := len(service.waitingPlayers)
	service.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("waiting player count = %d, want 0", remaining)
	}
}

func TestGameServiceMatchmakingRejectsInvalidTimeControl(t *testing.T) {
	t.Parallel()

	_, err := NewGameService().EnterMatchmaking(
		context.Background(),
		NewEnterMatchmakingCommand("player", 0, 0),
	)
	if !errors.Is(err, ErrInvalidTimeControl) {
		t.Fatalf("EnterMatchmaking() error = %v, want %v", err, ErrInvalidTimeControl)
	}
}

func TestAssignMatchmakingColorsRejectsInvalidColor(t *testing.T) {
	t.Parallel()

	if _, _, err := assignMatchmakingColors("waiting", "current", chess.NoColor); !errors.Is(err, ErrInvalidColorPreference) {
		t.Fatalf("assignMatchmakingColors() error = %v, want %v", err, ErrInvalidColorPreference)
	}
}

func assertNoMatch(t *testing.T, result <-chan MatchResult) {
	t.Helper()

	select {
	case match, ok := <-result:
		t.Fatalf("match result = (%+v, open=%v), want no result", match, ok)
	default:
	}
}

func receiveMatch(t *testing.T, result <-chan MatchResult) MatchResult {
	t.Helper()

	select {
	case match, ok := <-result:
		if !ok {
			t.Fatal("match result channel closed without a result")
		}
		return match
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for match result")
		return MatchResult{}
	}
}
