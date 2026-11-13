package persistence

import (
	"context"
	"fmt"

	"ChessLI/internal/identity"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=game_repository.go -destination=game_repository_mock.go -package=persistence

type GameRepository interface {
	CreateGame(ctx context.Context, game CompletedGame) error
	GetGameByID(ctx context.Context, gameID identity.GameID) (CompletedGame, error)
	ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]Summary, error)
}

type GameRepositoryImpl struct {
	db   *pgxpool.Pool
	psql sq.StatementBuilderType
}

// NewGameRepository returns a game repository backed by db.
func NewGameRepository(db *pgxpool.Pool) *GameRepositoryImpl {
	return &GameRepositoryImpl{db: db, psql: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

// CreateGame stores a completed game and its moves in a single insert.
// Move plies must start at one and be consecutive. Retrying the same game ID
// leaves the original immutable record intact.
func (r *GameRepositoryImpl) CreateGame(ctx context.Context, game CompletedGame) error {
	id, err := parseGameID(game.ID)
	if err != nil {
		return err
	}
	whiteProfileID, err := parseProfileID(game.WhiteProfileID)
	if err != nil {
		return err
	}
	blackProfileID, err := parseProfileID(game.BlackProfileID)
	if err != nil {
		return err
	}
	for i, move := range game.Moves {
		if move.Ply != i+1 {
			return fmt.Errorf("move ply %d: want %d", move.Ply, i+1)
		}
	}
	if game.Moves == nil {
		game.Moves = []Move{}
	}

	query, args, err := r.psql.Insert("completed_games").
		Columns("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
			"outcome", "termination", "final_fen", "white_remaining_ms", "black_remaining_ms", "completed_at", "moves").
		Values(id, whiteProfileID, blackProfileID, game.InitialTimeMS, game.IncrementMS,
			string(game.Outcome), string(game.Termination), game.FinalFEN,
			game.WhiteRemainingMS, game.BlackRemainingMS, game.CompletedAt, game.Moves).
		Suffix("ON CONFLICT (id) DO NOTHING").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, query, args...)
	return err
}

// GetGameByID returns a completed game and its moves by ID.
func (r *GameRepositoryImpl) GetGameByID(ctx context.Context, gameID identity.GameID) (CompletedGame, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return CompletedGame{}, err
	}

	query, args, err := r.psql.Select("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
		"outcome", "termination", "final_fen", "white_remaining_ms", "black_remaining_ms", "completed_at", "moves").
		From("completed_games").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return CompletedGame{}, err
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return CompletedGame{}, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CompletedGame])
}

// ListGamesByProfileID returns a page of completed games, newest first.
func (r *GameRepositoryImpl) ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]Summary, error) {
	id, err := parseProfileID(profileID)
	if err != nil {
		return nil, err
	}

	query, args, err := r.psql.Select("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
		"outcome", "termination", "completed_at").From("completed_games").
		Where(sq.Or{sq.Eq{"white_profile_id": id}, sq.Eq{"black_profile_id": id}}).
		OrderBy("completed_at DESC", "id DESC").
		Suffix("LIMIT ? OFFSET ?", pageSize, pageOffset).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Summary])
}

func parseGameID(id identity.GameID) (uuid.UUID, error) {
	parsed, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse game ID: %w", err)
	}
	return parsed, nil
}

func parseProfileID(id identity.ProfileID) (uuid.UUID, error) {
	parsed, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse profile ID: %w", err)
	}
	return parsed, nil
}
