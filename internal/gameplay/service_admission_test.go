package gameplay

import (
	"context"
	"errors"
	"testing"
	"time"

	"ChessLI/internal/identity"
)

func TestGameServiceConcurrentCreateReservesProfileOnce(t *testing.T) {
	s := NewGameService()
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
	s := NewGameService()
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
	match := receiveMatch(t, first.Result)
	receiveMatch(t, second.Result)
	if s.ActiveGame("first") != match.GameID || s.ActiveGame("second") != match.GameID {
		t.Fatal("matchmaking did not reserve both profiles")
	}
	if _, err = create("first"); !errors.Is(err, ErrAlreadyInGame) {
		t.Fatalf("matchmaking to private error = %v", err)
	}
}

func TestGameServiceActiveGameReleasesFinishedOrExpired(t *testing.T) {
	for _, name := range []string{"resigned", "expired"} {
		t.Run(name, func(t *testing.T) {
			s := NewGameService()
			defer s.Close()
			g := newReadyGame()
			s.games[g.state.id] = g
			s.profileGames["white"], s.profileGames["black"] = g.state.id, g.state.id
			if name == "expired" {
				g.now = func() time.Time { return g.state.clock.turnStartedAt.Add(10 * time.Minute) }
			} else if _, err := s.Resign(context.Background(), ResignCommand{GameID: g.state.id, ProfileID: "white"}); err != nil {
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
