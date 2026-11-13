package gameplay

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

func TestGameJoinPrivate(t *testing.T) {
	t.Parallel()

	t.Run("fills white seat", func(t *testing.T) {
		t.Parallel()

		game := newGame("game", "", "black", time.Minute, 0)
		color, err := game.joinPrivate(JoinPrivateCommand{ProfileID: "white"})
		if err != nil {
			t.Fatalf("JoinPrivate() error = %v", err)
		}
		if color != chess.White {
			t.Fatalf("JoinPrivate() color = %v, want %v", color, chess.White)
		}
		if game.whiteProfileID != "white" {
			t.Fatalf("WhiteProfileID = %q, want %q", game.whiteProfileID, "white")
		}
	})

	t.Run("fills black seat", func(t *testing.T) {
		t.Parallel()

		game := newGame("game", "white", "", time.Minute, 0)
		color, err := game.joinPrivate(JoinPrivateCommand{ProfileID: "black"})
		if err != nil {
			t.Fatalf("JoinPrivate() error = %v", err)
		}
		if color != chess.Black {
			t.Fatalf("JoinPrivate() color = %v, want %v", color, chess.Black)
		}
	})

	t.Run("rejects existing participant", func(t *testing.T) {
		t.Parallel()

		game := newGame("game", "white", "", time.Minute, 0)
		_, err := game.joinPrivate(JoinPrivateCommand{ProfileID: "white"})
		if !errors.Is(err, ErrAlreadyParticipant) {
			t.Fatalf("JoinPrivate() error = %v, want %v", err, ErrAlreadyParticipant)
		}
	})

	t.Run("rejects full game", func(t *testing.T) {
		t.Parallel()

		game := newReadyGame()
		_, err := game.joinPrivate(JoinPrivateCommand{ProfileID: "third"})
		if !errors.Is(err, ErrGameFull) {
			t.Fatalf("JoinPrivate() error = %v, want %v", err, ErrGameFull)
		}
	})
}

func TestGameMoveRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		game    func() *game
		command MoveCommand
		wantErr error
	}{
		{
			name: "game not ready",
			game: func() *game {
				return newGame("game", "white", "", time.Minute, 0)
			},
			command: MoveCommand{ProfileID: "white", Move: "e2e4", Notation: MoveNotationUCI},
			wantErr: ErrGameNotReady,
		},
		{
			name:    "stale version",
			game:    newReadyGame,
			command: MoveCommand{ProfileID: "white", Move: "e2e4", Notation: MoveNotationUCI, ExpectedVersion: 1},
			wantErr: ErrStaleGameVersion,
		},
		{
			name:    "wrong player",
			game:    newReadyGame,
			command: MoveCommand{ProfileID: "black", Move: "e7e5", Notation: MoveNotationUCI},
			wantErr: ErrNotYourTurn,
		},
		{
			name:    "non-participant",
			game:    newReadyGame,
			command: MoveCommand{ProfileID: "spectator", Move: "e2e4", Notation: MoveNotationUCI},
			wantErr: ErrNotParticipant,
		},
		{
			name:    "illegal move",
			game:    newReadyGame,
			command: MoveCommand{ProfileID: "white", Move: "e2e5", Notation: MoveNotationUCI},
			wantErr: ErrIllegalMove,
		},
		{
			name:    "unsupported notation",
			game:    newReadyGame,
			command: MoveCommand{ProfileID: "white", Move: "e2e4", Notation: MoveNotation(99)},
			wantErr: ErrUnsupportedNotation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.game().move(tt.command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Move() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGameMoveUpdatesAuthoritativeState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		move     string
		notation MoveNotation
	}{
		{name: "UCI", move: "e2e4", notation: MoveNotationUCI},
		{name: "SAN", move: "e4", notation: MoveNotationSAN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			game := newReadyGame()
			result, err := game.move(MoveCommand{
				GameID:    game.id,
				ProfileID: "white",
				Move:      tt.move,
				Notation:  tt.notation,
			})
			if err != nil {
				t.Fatalf("Move() error = %v", err)
			}
			if result.Version != 1 {
				t.Fatalf("Move() version = %d, want 1", result.Version)
			}
			if result.LastMoveSAN != "e4" {
				t.Fatalf("Move() LastMoveSAN = %q, want %q", result.LastMoveSAN, "e4")
			}

			snapshot := game.snapshot(game.now())
			if snapshot.FEN != result.FEN {
				t.Fatalf("Snapshot().FEN = %q, want %q", snapshot.FEN, result.FEN)
			}
			if snapshot.Version != result.Version {
				t.Fatalf("Snapshot().Version = %d, want %d", snapshot.Version, result.Version)
			}
			if snapshot.LastMoveSAN != result.LastMoveSAN {
				t.Fatalf("Snapshot().LastMoveSAN = %q, want %q", snapshot.LastMoveSAN, result.LastMoveSAN)
			}
		})
	}
}

func TestGameRejectsMoveAfterCheckmate(t *testing.T) {
	t.Parallel()

	game := newReadyGame()
	moves := []struct {
		profile identity.ProfileID
		move    string
	}{
		{profile: "white", move: "f2f3"},
		{profile: "black", move: "e7e5"},
		{profile: "white", move: "g2g4"},
		{profile: "black", move: "d8h4"},
	}

	for version, move := range moves {
		result, err := game.move(MoveCommand{
			GameID:          game.id,
			ProfileID:       move.profile,
			Move:            move.move,
			Notation:        MoveNotationUCI,
			ExpectedVersion: uint64(version),
		})
		if err != nil {
			t.Fatalf("Move(%q) error = %v", move.move, err)
		}

		if version == len(moves)-1 {
			if result.Outcome != domain.OutcomeBlackWin {
				t.Fatalf("final outcome = %v, want %v", result.Outcome, domain.OutcomeBlackWin)
			}
			if result.Termination != domain.TerminationCheckmate {
				t.Fatalf("final termination = %v, want %v", result.Termination, domain.TerminationCheckmate)
			}
		}
	}

	_, err := game.move(MoveCommand{
		ProfileID:       "white",
		Move:            "e2e4",
		Notation:        MoveNotationUCI,
		ExpectedVersion: uint64(len(moves)),
	})
	if !errors.Is(err, ErrGameFinished) {
		t.Fatalf("Move() error = %v, want %v", err, ErrGameFinished)
	}
}

func TestGamePrivateClockStartsWhenSecondPlayerJoins(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := newGame("game", "white", "", time.Minute, 0)
	game.now = func() time.Time { return now }

	if _, err := game.joinPrivate(JoinPrivateCommand{ProfileID: "black"}); err != nil {
		t.Fatalf("JoinPrivate() error = %v", err)
	}

	now = now.Add(5 * time.Second)
	snapshot := game.snapshot(game.now())
	if snapshot.WhiteRemaining != 55*time.Second {
		t.Fatalf("WhiteRemaining = %v, want 55s", snapshot.WhiteRemaining)
	}
	if snapshot.BlackRemaining != time.Minute {
		t.Fatalf("BlackRemaining = %v, want 1m", snapshot.BlackRemaining)
	}
}

func TestGameMoveCommitsClockAndIncrement(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := newGame("game", "white", "black", time.Minute, 2*time.Second)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Minute, 2*time.Second)
	game.clock.start(now)

	now = now.Add(5 * time.Second)
	snapshot, err := game.move(MoveCommand{
		GameID:    game.id,
		ProfileID: "white",
		Move:      "e2e4",
		Notation:  MoveNotationUCI,
	})
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if snapshot.WhiteRemaining != 57*time.Second {
		t.Fatalf("WhiteRemaining = %v, want 57s", snapshot.WhiteRemaining)
	}
	if snapshot.BlackRemaining != time.Minute {
		t.Fatalf("BlackRemaining = %v, want 1m", snapshot.BlackRemaining)
	}
}

func TestGameSnapshotLeavesTimeoutForExplicitExpiration(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := newGame("game", "white", "black", time.Second, 0)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)

	now = now.Add(time.Second)
	snapshot := game.snapshot(game.now())
	if snapshot.Outcome != domain.OutcomeNone || snapshot.Version != 0 || !game.clock.running {
		t.Fatalf("snapshot settled an expired clock: %+v", snapshot)
	}
	if !game.expire(now) {
		t.Fatal("explicit expiration did not settle the clock")
	}
	snapshot = game.snapshot(game.now())

	if snapshot.Outcome != domain.OutcomeBlackWin {
		t.Fatalf("Outcome = %v, want %v", snapshot.Outcome, domain.OutcomeBlackWin)
	}
	if snapshot.Termination != domain.TerminationTimeout {
		t.Fatalf("Termination = %v, want %v", snapshot.Termination, domain.TerminationTimeout)
	}
	if snapshot.Version != 1 {
		t.Fatalf("Version = %d, want 1", snapshot.Version)
	}
	if snapshot.WhiteRemaining != 0 {
		t.Fatalf("WhiteRemaining = %v, want 0", snapshot.WhiteRemaining)
	}

}

func TestGameTimeoutIsDrawWhenOpponentHasBareKing(t *testing.T) {
	t.Parallel()

	position, err := chess.FEN("8/8/8/8/8/2k5/8/R2K4 w - - 0 1")
	if err != nil {
		t.Fatalf("FEN() error = %v", err)
	}

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := newGame("game", "white", "black", time.Second, 0)
	game.engine = chess.NewGame(position)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)

	now = now.Add(time.Second)
	game.expire(now)
	snapshot := game.snapshot(game.now())
	if snapshot.Outcome != domain.OutcomeDraw || snapshot.Termination != domain.TerminationTimeout {
		t.Fatalf("timeout state = (%v, %v), want (%v, %v)", snapshot.Outcome, snapshot.Termination, domain.OutcomeDraw, domain.TerminationTimeout)
	}
}

func TestGameMoveReturnsTimeoutStateWithoutApplyingMove(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := newGame("game", "white", "black", time.Second, 0)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)
	startingFEN := game.engine.FEN()

	now = now.Add(time.Second)
	snapshot, err := game.move(MoveCommand{
		GameID:    game.id,
		ProfileID: "white",
		Move:      "e2e4",
		Notation:  MoveNotationUCI,
	})
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if snapshot.FEN != startingFEN || snapshot.LastMoveSAN != "" {
		t.Fatalf("timed-out move changed position: FEN=%q lastMove=%q", snapshot.FEN, snapshot.LastMoveSAN)
	}
	if snapshot.Outcome != domain.OutcomeBlackWin || snapshot.Termination != domain.TerminationTimeout {
		t.Fatalf("timeout state = (%v, %v), want (%v, %v)", snapshot.Outcome, snapshot.Termination, domain.OutcomeBlackWin, domain.TerminationTimeout)
	}
}

func newReadyGame() *game {
	return newGame("game", "white", "black", 10*time.Minute, 0)
}

func TestGameDrawOfferLifecycle(t *testing.T) {
	base := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	now := base
	game := newReadyGame()
	game.now = func() time.Time { return now }

	if _, err := game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "spectator"}); !errors.Is(err, ErrNotParticipant) {
		t.Fatalf("OfferDraw() spectator error = %v, want %v", err, ErrNotParticipant)
	}

	offered, err := game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "white"})
	if err != nil {
		t.Fatalf("OfferDraw() error = %v", err)
	}
	if offered.Version != 0 || offered.PendingDrawOffer == nil {
		t.Fatalf("OfferDraw() state = %+v, want pending offer at unchanged version", offered)
	}
	offerID := offered.PendingDrawOffer.OfferID
	if _, err = game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "black"}); !errors.Is(err, ErrDrawOfferPending) {
		t.Fatalf("OfferDraw() pending-offer error = %v, want %v", err, ErrDrawOfferPending)
	}

	if _, err = game.acceptDraw(DrawOfferResponseCommand{GameID: game.id, ProfileID: "white", OfferID: offerID}); !errors.Is(err, ErrCannotRespondToOwnDrawOffer) {
		t.Fatalf("AcceptDraw() own-offer error = %v, want %v", err, ErrCannotRespondToOwnDrawOffer)
	}
	if _, err = game.acceptDraw(DrawOfferResponseCommand{GameID: game.id, ProfileID: "black", OfferID: "stale"}); !errors.Is(err, ErrStaleDrawOffer) {
		t.Fatalf("AcceptDraw() stale-offer error = %v, want %v", err, ErrStaleDrawOffer)
	}

	declined, err := game.declineDraw(DrawOfferResponseCommand{GameID: game.id, ProfileID: "black", OfferID: offerID})
	if err != nil {
		t.Fatalf("DeclineDraw() error = %v", err)
	}
	if declined.Version != 0 || declined.PendingDrawOffer != nil {
		t.Fatalf("DeclineDraw() state = %+v, want cleared offer at unchanged version", declined)
	}
	if _, err = game.declineDraw(DrawOfferResponseCommand{GameID: game.id, ProfileID: "black", OfferID: offerID}); !errors.Is(err, ErrDrawOfferNotFound) {
		t.Fatalf("DeclineDraw() missing-offer error = %v, want %v", err, ErrDrawOfferNotFound)
	}
	if _, err = game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "white"}); !errors.Is(err, ErrDrawOfferCooldown) {
		t.Fatalf("OfferDraw() cooldown error = %v, want %v", err, ErrDrawOfferCooldown)
	}

	now = now.Add(drawOfferCooldown)
	offered, err = game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "white"})
	if err != nil {
		t.Fatalf("OfferDraw() after cooldown error = %v", err)
	}
	offerID = offered.PendingDrawOffer.OfferID
	offered.PendingDrawOffer.OfferedBy = "tampered"
	if got := game.snapshot(game.now()).PendingDrawOffer.OfferedBy; got != "white" {
		t.Fatalf("Snapshot() pending offer was aliased: OfferedBy = %q", got)
	}

	accepted, err := game.acceptDraw(DrawOfferResponseCommand{GameID: game.id, ProfileID: "black", OfferID: offerID})
	if err != nil {
		t.Fatalf("AcceptDraw() error = %v", err)
	}
	if accepted.Version != 1 || accepted.Outcome != domain.OutcomeDraw || accepted.Termination != domain.TerminationDrawAgreement || accepted.PendingDrawOffer != nil {
		t.Fatalf("AcceptDraw() state = %+v, want finished draw at version 1", accepted)
	}
}

func TestGameClearsPendingDrawOffer(t *testing.T) {
	tests := []struct {
		name string
		act  func(*game) (GameSnapshot, error)
	}{
		{
			name: "move",
			act: func(game *game) (GameSnapshot, error) {
				return game.move(MoveCommand{GameID: game.id, ProfileID: "white", Move: "e2e4", Notation: MoveNotationUCI})
			},
		},
		{
			name: "resignation",
			act: func(game *game) (GameSnapshot, error) {
				return game.resign(ResignCommand{GameID: game.id, ProfileID: "black"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := newReadyGame()
			if _, err := game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "white"}); err != nil {
				t.Fatalf("OfferDraw() error = %v", err)
			}

			state, err := tt.act(game)
			if err != nil {
				t.Fatalf("action error = %v", err)
			}
			if state.PendingDrawOffer != nil {
				t.Fatalf("action left pending offer %+v", state.PendingDrawOffer)
			}
		})
	}

	t.Run("timeout", func(t *testing.T) {
		base := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
		now := base
		game := newGame("game", "white", "black", time.Second, 0)
		game.now = func() time.Time { return now }
		game.clock = newGameClock(time.Second, 0)
		game.clock.start(now)
		if _, err := game.offerDraw(OfferDrawCommand{GameID: game.id, ProfileID: "white"}); err != nil {
			t.Fatalf("OfferDraw() error = %v", err)
		}

		now = now.Add(time.Second)
		game.expire(now)
		if state := game.snapshot(game.now()); state.PendingDrawOffer != nil {
			t.Fatalf("timeout left pending offer %+v", state.PendingDrawOffer)
		}
	})
}

func TestGameLogsCompletionOnce(t *testing.T) {
	for _, ending := range []domain.Termination{
		domain.TerminationCheckmate, domain.TerminationResignation,
		domain.TerminationDrawAgreement, domain.TerminationTimeout,
	} {
		t.Run(string(ending), func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			g := newReadyGame()
			switch ending {
			case domain.TerminationCheckmate:
				for i, move := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
					player := identity.ProfileID("white")
					if i%2 == 1 {
						player = "black"
					}
					if _, err := g.move(MoveCommand{ProfileID: player, Move: move, ExpectedVersion: uint64(i)}); err != nil {
						t.Fatal(err)
					}
				}
			case domain.TerminationResignation:
				if _, err := g.resign(ResignCommand{ProfileID: "white"}); err != nil {
					t.Fatal(err)
				}
			case domain.TerminationDrawAgreement:
				state, err := g.offerDraw(OfferDrawCommand{ProfileID: "white"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := g.acceptDraw(DrawOfferResponseCommand{ProfileID: "black", OfferID: state.PendingDrawOffer.OfferID}); err != nil {
					t.Fatal(err)
				}
			case domain.TerminationTimeout:
				now := g.clock.turnStartedAt.Add(time.Hour)
				g.now = func() time.Time { return now }
				g.expire(now)
			}
			state := g.snapshot(g.now())
			for range 2 {
				if _, err := g.resume("white"); err != nil {
					t.Fatal(err)
				}
				g.snapshot(g.now())
			}
			finished := 0
			for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
				if len(line) == 0 {
					continue
				}
				var event struct {
					Event       string
					GameID      identity.GameID `json:"game_id"`
					Outcome     domain.Outcome
					Termination domain.Termination
					Version     uint64
				}
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal(err)
				}
				if event.Event == "game.finished" {
					finished++
					if event.GameID != g.id || event.Outcome != state.Outcome || event.Termination != ending || event.Version != state.Version {
						t.Fatalf("completion event = %+v, state = %+v", event, state)
					}
				}
			}
			if finished != 1 {
				t.Fatalf("completion events = %d, want one despite repeated reads", finished)
			}
		})
	}
}
