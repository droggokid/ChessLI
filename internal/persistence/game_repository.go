package persistence

import (
	"context"
	"fmt"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=game_repository_mock.go -package=persistence -mock_names=Repository=MockGameRepository ChessLI/internal/persistence GameRepository

type GameRepository interface {
	CreateGame(ctx context.Context, game domain.CompletedGame) error
	CreateMoves(ctx context.Context, gameID identity.GameID, moves []domain.Move) error
	GetGameByID(ctx context.Context, gameID identity.GameID) (domain.CompletedGame, error)
	ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]domain.Summary, error)
	ListMovesByGameID(ctx context.Context, gameID identity.GameID) ([]domain.Move, error)
}

type GameRepositoryImpl struct {
	db   *pgxpool.Pool
	psql sq.StatementBuilderType
}

// NewGameRepository returns a game repository backed by db.
func NewGameRepository(db *pgxpool.Pool) *GameRepositoryImpl {
	return &GameRepositoryImpl{db: db, psql: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

// CreateGame stores a completed game without its moves.
func (r *GameRepositoryImpl) CreateGame(ctx context.Context, game domain.CompletedGame) error {
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

	query, args, err := r.psql.Insert("completed_games").
		Columns("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
			"outcome", "termination", "final_fen", "completed_at").
		Values(id, whiteProfileID, blackProfileID, game.Initial.Milliseconds(), game.Increment.Milliseconds(),
			string(game.Outcome), string(game.Termination), game.FinalFEN, game.CompletedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, query, args...)
	return err
}

// CreateMoves stores moves for an existing game; plies must start at one and be consecutive.
// shortcut: game and move writes are separate, add an atomic save when wiring persistence.
func (r *GameRepositoryImpl) CreateMoves(ctx context.Context, gameID identity.GameID, moves []domain.Move) error {
	id, err := parseGameID(gameID)
	if err != nil {
		return err
	}

	whiteRemaining := make([]int64, len(moves))
	blackRemaining := make([]int64, len(moves))
	uciMoves := make([]string, len(moves))
	for i, move := range moves {
		if move.Ply != i+1 {
			return fmt.Errorf("move ply %d: want %d", move.Ply, i+1)
		}
		whiteRemaining[i] = move.WhiteRemaining.Milliseconds()
		blackRemaining[i] = move.BlackRemaining.Milliseconds()
		uciMoves[i] = move.UCI
	}

	query, args, err := r.psql.Insert("completed_game_moves").
		Columns("game_id", "ply", "uci", "white_remaining_ms", "black_remaining_ms").
		Select(sq.Select().Column("?::uuid", id).
			Column("moves.ply::integer").Column("moves.uci").
			Column("(?::bigint[])[moves.ply::integer]", whiteRemaining).
			Column("(?::bigint[])[moves.ply::integer]", blackRemaining).
			Suffix("FROM unnest(?::text[]) WITH ORDINALITY AS moves(uci, ply)", uciMoves)).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, query, args...)
	return err
}

// GetGameByID returns a completed game by ID, without its moves.
func (r *GameRepositoryImpl) GetGameByID(ctx context.Context, gameID identity.GameID) (domain.CompletedGame, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return domain.CompletedGame{}, err
	}

	query, args, err := r.psql.Select("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
		"outcome", "termination", "final_fen", "completed_at").
		From("completed_games").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return domain.CompletedGame{}, err
	}
	var game domain.CompletedGame
	var storedID, whiteID, blackID uuid.UUID
	var initialMS, incrementMS int64
	if err = r.db.QueryRow(ctx, query, args...).Scan(&storedID, &whiteID, &blackID, &initialMS, &incrementMS,
		&game.Outcome, &game.Termination, &game.FinalFEN, &game.CompletedAt); err != nil {
		return domain.CompletedGame{}, err
	}
	game.ID = identity.GameID(storedID.String())
	game.WhiteProfileID = identity.ProfileID(whiteID.String())
	game.BlackProfileID = identity.ProfileID(blackID.String())
	game.Initial = durationFromMilliseconds(initialMS)
	game.Increment = durationFromMilliseconds(incrementMS)
	return game, nil
}

// ListGamesByProfileID returns a page of completed games, newest first.
func (r *GameRepositoryImpl) ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]domain.Summary, error) {
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
	defer rows.Close()
	games := make([]domain.Summary, 0)
	for rows.Next() {
		var game domain.Summary
		var gameID, whiteID, blackID uuid.UUID
		var initialMS, incrementMS int64
		if err := rows.Scan(&gameID, &whiteID, &blackID, &initialMS, &incrementMS,
			&game.Outcome, &game.Termination, &game.CompletedAt); err != nil {
			return nil, err
		}
		game.ID = identity.GameID(gameID.String())
		game.WhiteProfileID = identity.ProfileID(whiteID.String())
		game.BlackProfileID = identity.ProfileID(blackID.String())
		game.Initial = durationFromMilliseconds(initialMS)
		game.Increment = durationFromMilliseconds(incrementMS)
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return games, nil
}

// ListMovesByGameID returns a game's moves in ascending ply order.
func (r *GameRepositoryImpl) ListMovesByGameID(ctx context.Context, gameID identity.GameID) ([]domain.Move, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return nil, err
	}

	query, args, err := r.psql.Select("ply", "uci", "white_remaining_ms", "black_remaining_ms").
		From("completed_game_moves").Where(sq.Eq{"game_id": id}).
		OrderBy("ply").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	moves := make([]domain.Move, 0)
	for rows.Next() {
		var move domain.Move
		var whiteMS, blackMS int64
		if err := rows.Scan(&move.Ply, &move.UCI, &whiteMS, &blackMS); err != nil {
			return nil, err
		}
		move.WhiteRemaining = durationFromMilliseconds(whiteMS)
		move.BlackRemaining = durationFromMilliseconds(blackMS)
		moves = append(moves, move)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return moves, nil
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

func durationFromMilliseconds(milliseconds int64) time.Duration {
	return time.Duration(milliseconds) * time.Millisecond
}
