package game

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ChessLI/internal/identity"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=repository.go -destination=repository_mock.go -package=game

type Repository interface {
	CreateGame(ctx context.Context, game CompletedGame) error
	GetGameByID(ctx context.Context, gameID identity.GameID) (CompletedGame, error)
	ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]Summary, error)
}

type RepositoryImpl struct {
	db   *pgxpool.Pool
	psql sq.StatementBuilderType
}

// NewRepository returns a game repository backed by db.
func NewRepository(db *pgxpool.Pool) *RepositoryImpl {
	return &RepositoryImpl{db: db, psql: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

// CreateGame stores a completed game and its moves in a single insert.
// Move plies must start at one and be consecutive.
func (r *RepositoryImpl) CreateGame(ctx context.Context, game CompletedGame) error {
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
	moves, err := encodeMoves(game.Moves)
	if err != nil {
		return err
	}

	query, args, err := r.psql.Insert("completed_games").
		Columns("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
			"outcome", "termination", "final_fen", "completed_at", "moves").
		Values(id, whiteProfileID, blackProfileID, game.Initial.Milliseconds(), game.Increment.Milliseconds(),
			string(game.Outcome), string(game.Termination), game.FinalFEN, game.CompletedAt, moves).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, query, args...)
	return err
}

// GetGameByID returns a completed game and its moves by ID.
func (r *RepositoryImpl) GetGameByID(ctx context.Context, gameID identity.GameID) (CompletedGame, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return CompletedGame{}, err
	}

	query, args, err := r.psql.Select("id", "white_profile_id", "black_profile_id", "initial_time_ms", "increment_ms",
		"outcome", "termination", "final_fen", "completed_at", "moves").
		From("completed_games").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return CompletedGame{}, err
	}
	var game CompletedGame
	var storedID, whiteID, blackID uuid.UUID
	var initialMS, incrementMS int64
	var moves []byte
	if err = r.db.QueryRow(ctx, query, args...).Scan(&storedID, &whiteID, &blackID, &initialMS, &incrementMS,
		&game.Outcome, &game.Termination, &game.FinalFEN, &game.CompletedAt, &moves); err != nil {
		return CompletedGame{}, err
	}
	game.Moves, err = decodeMoves(moves)
	if err != nil {
		return CompletedGame{}, fmt.Errorf("decode game moves: %w", err)
	}
	game.ID = identity.GameID(storedID.String())
	game.WhiteProfileID = identity.ProfileID(whiteID.String())
	game.BlackProfileID = identity.ProfileID(blackID.String())
	game.Initial = durationFromMilliseconds(initialMS)
	game.Increment = durationFromMilliseconds(incrementMS)
	return game, nil
}

// ListGamesByProfileID returns a page of completed games, newest first.
func (r *RepositoryImpl) ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]Summary, error) {
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
	games := make([]Summary, 0)
	for rows.Next() {
		var game Summary
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

type storedMove struct {
	Ply              int    `json:"ply"`
	UCI              string `json:"uci"`
	WhiteRemainingMS int64  `json:"white_remaining_ms"`
	BlackRemainingMS int64  `json:"black_remaining_ms"`
}

func encodeMoves(moves []Move) ([]byte, error) {
	stored := make([]storedMove, len(moves))
	for i, move := range moves {
		if move.Ply != i+1 {
			return nil, fmt.Errorf("move ply %d: want %d", move.Ply, i+1)
		}
		stored[i] = storedMove{
			Ply:              move.Ply,
			UCI:              move.UCI,
			WhiteRemainingMS: move.WhiteRemaining.Milliseconds(),
			BlackRemainingMS: move.BlackRemaining.Milliseconds(),
		}
	}
	return json.Marshal(stored)
}

func decodeMoves(data []byte) ([]Move, error) {
	var stored []storedMove
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	moves := make([]Move, len(stored))
	for i, move := range stored {
		moves[i] = Move{
			Ply:            move.Ply,
			UCI:            move.UCI,
			WhiteRemaining: durationFromMilliseconds(move.WhiteRemainingMS),
			BlackRemaining: durationFromMilliseconds(move.BlackRemainingMS),
		}
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
