package gameplay

import (
	"errors"
	"testing"
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

func TestGameJoinPrivate(t *testing.T) {
	t.Parallel()

	t.Run("fills white seat", func(t *testing.T) {
		t.Parallel()

		game := NewGame("game", "", "black", time.Minute, 0)
		color, err := game.JoinPrivate(JoinPrivateCommand{ProfileID: "white"})
		if err != nil {
			t.Fatalf("JoinPrivate() error = %v", err)
		}
		if color != chess.White {
			t.Fatalf("JoinPrivate() color = %v, want %v", color, chess.White)
		}
		if game.WhiteProfileID != "white" {
			t.Fatalf("WhiteProfileID = %q, want %q", game.WhiteProfileID, "white")
		}
	})

	t.Run("fills black seat", func(t *testing.T) {
		t.Parallel()

		game := NewGame("game", "white", "", time.Minute, 0)
		color, err := game.JoinPrivate(JoinPrivateCommand{ProfileID: "black"})
		if err != nil {
			t.Fatalf("JoinPrivate() error = %v", err)
		}
		if color != chess.Black {
			t.Fatalf("JoinPrivate() color = %v, want %v", color, chess.Black)
		}
	})

	t.Run("rejects existing participant", func(t *testing.T) {
		t.Parallel()

		game := NewGame("game", "white", "", time.Minute, 0)
		_, err := game.JoinPrivate(JoinPrivateCommand{ProfileID: "white"})
		if !errors.Is(err, ErrAlreadyParticipant) {
			t.Fatalf("JoinPrivate() error = %v, want %v", err, ErrAlreadyParticipant)
		}
	})

	t.Run("rejects full game", func(t *testing.T) {
		t.Parallel()

		game := newReadyGame()
		_, err := game.JoinPrivate(JoinPrivateCommand{ProfileID: "third"})
		if !errors.Is(err, ErrGameFull) {
			t.Fatalf("JoinPrivate() error = %v, want %v", err, ErrGameFull)
		}
	})
}

func TestGameMoveRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		game    func() *Game
		command MoveCommand
		wantErr error
	}{
		{
			name: "game not ready",
			game: func() *Game {
				return NewGame("game", "white", "", time.Minute, 0)
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

			_, err := tt.game().Move(tt.command)
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
			result, err := game.Move(MoveCommand{
				GameID:    game.ID,
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

			snapshot := game.Snapshot()
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
		result, err := game.Move(MoveCommand{
			GameID:          game.ID,
			ProfileID:       move.profile,
			Move:            move.move,
			Notation:        MoveNotationUCI,
			ExpectedVersion: uint64(version),
		})
		if err != nil {
			t.Fatalf("Move(%q) error = %v", move.move, err)
		}

		if version == len(moves)-1 {
			if result.Outcome != chess.BlackWon {
				t.Fatalf("final outcome = %v, want %v", result.Outcome, chess.BlackWon)
			}
			if result.Termination != TerminationCheckmate {
				t.Fatalf("final termination = %v, want %v", result.Termination, TerminationCheckmate)
			}
		}
	}

	_, err := game.Move(MoveCommand{
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
	game := NewGame("game", "white", "", time.Minute, 0)
	game.now = func() time.Time { return now }

	if _, err := game.JoinPrivate(JoinPrivateCommand{ProfileID: "black"}); err != nil {
		t.Fatalf("JoinPrivate() error = %v", err)
	}

	now = now.Add(5 * time.Second)
	snapshot := game.Snapshot()
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
	game := NewGame("game", "white", "black", time.Minute, 2*time.Second)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Minute, 2*time.Second)
	game.clock.start(now)

	now = now.Add(5 * time.Second)
	snapshot, err := game.Move(MoveCommand{
		GameID:    game.ID,
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

func TestGameSnapshotAdjudicatesTimeout(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := NewGame("game", "white", "black", time.Second, 0)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)

	now = now.Add(time.Second)
	snapshot := game.Snapshot()

	if snapshot.Outcome != chess.BlackWon {
		t.Fatalf("Outcome = %v, want %v", snapshot.Outcome, chess.BlackWon)
	}
	if snapshot.Termination != TerminationTimeout {
		t.Fatalf("Termination = %v, want %v", snapshot.Termination, TerminationTimeout)
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
	game := NewGame("game", "white", "black", time.Second, 0)
	game.engine = chess.NewGame(position)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)

	now = now.Add(time.Second)
	snapshot := game.Snapshot()
	if snapshot.Outcome != chess.Draw || snapshot.Termination != TerminationTimeout {
		t.Fatalf("timeout state = (%v, %v), want (%v, %v)", snapshot.Outcome, snapshot.Termination, chess.Draw, TerminationTimeout)
	}
}

func TestGameMoveReturnsTimeoutStateWithoutApplyingMove(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := base
	game := NewGame("game", "white", "black", time.Second, 0)
	game.now = func() time.Time { return now }
	game.clock = newGameClock(time.Second, 0)
	game.clock.start(now)
	startingFEN := game.engine.FEN()

	now = now.Add(time.Second)
	snapshot, err := game.Move(MoveCommand{
		GameID:    game.ID,
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
	if snapshot.Outcome != chess.BlackWon || snapshot.Termination != TerminationTimeout {
		t.Fatalf("timeout state = (%v, %v), want (%v, %v)", snapshot.Outcome, snapshot.Termination, chess.BlackWon, TerminationTimeout)
	}
}

func newReadyGame() *Game {
	return NewGame("game", "white", "black", 10*time.Minute, 0)
}
