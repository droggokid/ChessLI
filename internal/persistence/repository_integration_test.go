package persistence_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ChessLI/internal/database/sqlc"
	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
	gamestore "ChessLI/internal/persistence/game"
	profilestore "ChessLI/internal/persistence/profile"
	"ChessLI/internal/profile"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresProfileRoundTrip(t *testing.T) {
	ctx, tx := testDatabase(t)
	repository := profilestore.NewProfileRepository(*sqlc.New(tx))
	first, second := createProfiles(t, ctx, repository)
	got, err := repository.GetProfileByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first.ID || got.Username != "White" || got.Email != "white@example.test" || got.CreatedAt.IsZero() {
		t.Fatalf("profile round trip = %+v", got)
	}
	credential, err := repository.GetCredentialByEmail(ctx, "WHITE@EXAMPLE.TEST")
	if err != nil {
		t.Fatal(err)
	}
	if credential.ProfileID != first.ID || credential.PasswordHash != "test-hash" {
		t.Fatalf("credential = %+v", credential)
	}
	profiles, err := repository.GetProfilesByIDs(ctx, []identity.ProfileID{second.ID, identity.NewProfileID(), first.ID})
	if err != nil {
		t.Fatal(err)
	}
	ids := []identity.ProfileID{first.ID, second.ID}
	slices.Sort(ids)
	if len(profiles) != 2 || profiles[0].ID != ids[0] || profiles[1].ID != ids[1] {
		t.Fatalf("profiles = %+v, want sorted existing IDs %v", profiles, ids)
	}
	if _, err := repository.GetProfileByID(ctx, identity.NewProfileID()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing profile error = %v", err)
	}
}

func TestPostgresCompletedGameRoundTrip(t *testing.T) {
	ctx, tx := testDatabase(t)
	queries := sqlc.New(tx)
	white, black := createProfiles(t, ctx, profilestore.NewProfileRepository(*queries))
	repository := gamestore.NewGameRepository(*queries)
	completed := completedGame(white.ID, black.ID)
	if err := repository.CreateGame(ctx, completed); err != nil {
		t.Fatal(err)
	}
	moves := []domain.Move{
		{Ply: 1, UCI: "e2e4", WhiteRemaining: 59 * time.Second, BlackRemaining: time.Minute},
		{Ply: 2, UCI: "e7e5", WhiteRemaining: 59 * time.Second, BlackRemaining: 58 * time.Second},
	}
	if err := repository.CreateMoves(ctx, completed.ID, moves); err != nil {
		t.Fatal(err)
	}
	got, err := repository.GetGameByID(ctx, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CompletedAt.Equal(completed.CompletedAt) {
		t.Fatalf("completion time = %v", got.CompletedAt)
	}
	got.CompletedAt = completed.CompletedAt
	if got != completed {
		t.Fatalf("completed game = %+v, want %+v", got, completed)
	}
	gotMoves, err := repository.ListMovesByGameID(ctx, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotMoves, moves) {
		t.Fatalf("moves = %+v, want %+v", gotMoves, moves)
	}
	newer := completed
	newer.ID = identity.NewGameID()
	newer.CompletedAt = completed.CompletedAt.Add(time.Minute)
	if err := repository.CreateGame(ctx, newer); err != nil {
		t.Fatal(err)
	}
	for _, player := range []identity.ProfileID{white.ID, black.ID} {
		for offset, wantID := range []identity.GameID{newer.ID, completed.ID} {
			page, err := repository.ListGamesByProfileID(ctx, player, int32(offset), 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) != 1 || page[0].ID != wantID || page[0].Initial != time.Minute || page[0].Termination != domain.TerminationResignation {
				t.Fatalf("page %d = %+v", offset, page)
			}
		}
	}
	if _, err := repository.GetGameByID(ctx, identity.NewGameID()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing game error = %v", err)
	}
	if err := repository.CreateMoves(ctx, newer.ID, nil); err != nil {
		t.Fatal(err)
	}
	if empty, err := repository.ListMovesByGameID(ctx, newer.ID); err != nil || len(empty) != 0 {
		t.Fatalf("empty move history = %+v, %v", empty, err)
	}
}

func TestPostgresRejectsDuplicateProfiles(t *testing.T) {
	for _, field := range []string{"username", "email"} {
		t.Run(field, func(t *testing.T) {
			ctx, tx := testDatabase(t)
			repository := profilestore.NewProfileRepository(*sqlc.New(tx))
			createProfiles(t, ctx, repository)
			username, email := "other", "other@example.test"
			if field == "username" {
				username = "WHITE"
			} else {
				email = "WHITE@EXAMPLE.TEST"
			}
			_, err := repository.CreateProfile(ctx, username, email, "test-hash")
			assertSQLState(t, err, "23505")
		})
	}
}

func TestPostgresRejectsInvalidCompletedGames(t *testing.T) {
	for _, invalid := range []struct {
		name   string
		change func(*domain.CompletedGame)
		code   string
	}{
		{"missing player", func(g *domain.CompletedGame) { g.WhiteProfileID = identity.NewProfileID() }, "23503"},
		{"same players", func(g *domain.CompletedGame) { g.BlackProfileID = g.WhiteProfileID }, "23514"},
		{"active game", func(g *domain.CompletedGame) { g.Outcome = domain.OutcomeNone }, "23514"},
		{"invalid time control", func(g *domain.CompletedGame) { g.Initial = 0 }, "23514"},
		{"missing termination", func(g *domain.CompletedGame) { g.Termination = domain.TerminationNone }, "23514"},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			ctx, tx := testDatabase(t)
			queries := sqlc.New(tx)
			white, black := createProfiles(t, ctx, profilestore.NewProfileRepository(*queries))
			completed := completedGame(white.ID, black.ID)
			invalid.change(&completed)
			assertSQLState(t, gamestore.NewGameRepository(*queries).CreateGame(ctx, completed), invalid.code)
		})
	}
}

func TestPostgresRejectsInvalidMovesAndAllowsRollback(t *testing.T) {
	ctx, tx := testDatabase(t)
	queries := sqlc.New(tx)
	white, black := createProfiles(t, ctx, profilestore.NewProfileRepository(*queries))
	repository := gamestore.NewGameRepository(*queries)
	completed := completedGame(white.ID, black.ID)
	if err := repository.CreateGame(ctx, completed); err != nil {
		t.Fatal(err)
	}
	// A savepoint lets us inspect the game after PostgreSQL rejects the batch.
	batch, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = gamestore.NewGameRepository(*sqlc.New(batch)).CreateMoves(ctx, completed.ID, []domain.Move{
		{Ply: 1, UCI: "e2e4", WhiteRemaining: time.Minute, BlackRemaining: time.Minute},
		{Ply: 2, UCI: "e7e5", WhiteRemaining: -time.Second, BlackRemaining: time.Minute},
	})
	assertSQLState(t, err, "23514")
	if err := batch.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	moves, err := repository.ListMovesByGameID(ctx, completed.ID)
	if err != nil || len(moves) != 0 {
		t.Fatalf("rejected batch left moves = %+v, error %v", moves, err)
	}
}

func testDatabase(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	databaseURL := os.Getenv("CHESSLI_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set CHESSLI_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := conn.Close(cleanupCtx); err != nil {
			t.Error(err)
		}
	})
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Error(err)
		}
	})
	schema := pgx.Identifier{"chessli_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	if _, err := tx.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob("../database/migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, migration := range migrations {
		source, err := os.ReadFile(migration)
		if err != nil {
			t.Fatal(err)
		}
		up, _, found := strings.Cut(string(source), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Down boundary", migration)
		}
		if _, err := tx.Exec(ctx, up); err != nil {
			t.Fatalf("apply %s: %v", migration, err)
		}
	}
	return ctx, tx
}

func createProfiles(t *testing.T, ctx context.Context, repository *profilestore.ProfileRepository) (profile.Profile, profile.Profile) {
	t.Helper()
	white, err := repository.CreateProfile(ctx, "White", "white@example.test", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	black, err := repository.CreateProfile(ctx, "Black", "black@example.test", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	return white, black
}

func completedGame(white, black identity.ProfileID) domain.CompletedGame {
	return domain.CompletedGame{
		ID: identity.NewGameID(), WhiteProfileID: white, BlackProfileID: black,
		Initial: time.Minute, Increment: time.Second,
		Outcome: domain.OutcomeWhiteWin, Termination: domain.TerminationResignation,
		FinalFEN:    "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
		CompletedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
	}
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != want {
		t.Fatalf("database error = %v, want SQLSTATE %s", err, want)
	}
}
