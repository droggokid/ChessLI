package gameplay

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/persistence"

	"go.uber.org/mock/gomock"
)

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
