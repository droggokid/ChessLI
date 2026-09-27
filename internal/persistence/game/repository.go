package game

import (
	"context"
	"fmt"
	"time"

	"ChessLI/internal/database/sqlc"
	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=repository_mock.go -package=game -mock_names=Repository=MockGameRepository ChessLI/internal/persistence/game Repository

type Repository interface {
	CreateGame(ctx context.Context, game domain.Game) error
	CreateMoves(ctx context.Context, gameID identity.GameID, moves []domain.Move) error
	GetGameByID(ctx context.Context, gameID identity.GameID) (domain.Game, error)
	ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]domain.Summary, error)
	ListMovesByGameID(ctx context.Context, gameID identity.GameID) ([]domain.Move, error)
}

type GameRepository struct {
	queries *sqlc.Queries
}

func NewGameRepository(queries sqlc.Queries) *GameRepository {
	return &GameRepository{queries: &queries}
}

func (r *GameRepository) CreateGame(ctx context.Context, game domain.Game) error {
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

	return r.queries.CreateCompletedGame(ctx, sqlc.CreateCompletedGameParams{
		ID:             id,
		WhiteProfileID: whiteProfileID,
		BlackProfileID: blackProfileID,
		InitialTimeMs:  game.Initial.Milliseconds(),
		IncrementMs:    game.Increment.Milliseconds(),
		Outcome:        string(game.Outcome),
		Termination:    string(game.Termination),
		FinalFen:       game.FinalFEN,
		CompletedAt:    game.CompletedAt,
	})
}

// CreateMoves stores moves for a game already created with CreateGame.
// ponytail: writes are separate until persistence wiring supplies a transaction-capable dependency.
func (r *GameRepository) CreateMoves(ctx context.Context, gameID identity.GameID, moves []domain.Move) error {
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

	return r.queries.CreateCompletedGameMoves(ctx, sqlc.CreateCompletedGameMovesParams{
		GameID:           id,
		WhiteRemainingMs: whiteRemaining,
		BlackRemainingMs: blackRemaining,
		UciMoves:         uciMoves,
	})
}

func (r *GameRepository) GetGameByID(ctx context.Context, gameID identity.GameID) (domain.Game, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return domain.Game{}, err
	}

	row, err := r.queries.GetCompletedGameByID(ctx, id)
	if err != nil {
		return domain.Game{}, err
	}
	return gameFromRow(row), nil
}

func (r *GameRepository) ListGamesByProfileID(ctx context.Context, profileID identity.ProfileID, pageOffset, pageSize int32) ([]domain.Summary, error) {
	id, err := parseProfileID(profileID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.ListCompletedGamesByProfileID(ctx, sqlc.ListCompletedGamesByProfileIDParams{
		ProfileID:  id,
		PageOffset: pageOffset,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}

	games := make([]domain.Summary, len(rows))
	for i, row := range rows {
		games[i] = domain.Summary{
			ID:             identity.GameID(row.ID.String()),
			WhiteProfileID: identity.ProfileID(row.WhiteProfileID.String()),
			BlackProfileID: identity.ProfileID(row.BlackProfileID.String()),
			Initial:        durationFromMilliseconds(row.InitialTimeMs),
			Increment:      durationFromMilliseconds(row.IncrementMs),
			Outcome:        domain.Outcome(row.Outcome),
			Termination:    domain.Termination(row.Termination),
			CompletedAt:    row.CompletedAt,
		}
	}
	return games, nil
}

func (r *GameRepository) ListMovesByGameID(ctx context.Context, gameID identity.GameID) ([]domain.Move, error) {
	id, err := parseGameID(gameID)
	if err != nil {
		return nil, err
	}

	rows, err := r.queries.ListCompletedGameMovesByGameID(ctx, id)
	if err != nil {
		return nil, err
	}

	moves := make([]domain.Move, len(rows))
	for i, row := range rows {
		moves[i] = domain.Move{
			Ply:            int(row.Ply),
			UCI:            row.Uci,
			WhiteRemaining: durationFromMilliseconds(row.WhiteRemainingMs),
			BlackRemaining: durationFromMilliseconds(row.BlackRemainingMs),
		}
	}
	return moves, nil
}

func gameFromRow(row sqlc.CompletedGame) domain.Game {
	return domain.Game{
		ID:             identity.GameID(row.ID.String()),
		WhiteProfileID: identity.ProfileID(row.WhiteProfileID.String()),
		BlackProfileID: identity.ProfileID(row.BlackProfileID.String()),
		Initial:        durationFromMilliseconds(row.InitialTimeMs),
		Increment:      durationFromMilliseconds(row.IncrementMs),
		Outcome:        domain.Outcome(row.Outcome),
		Termination:    domain.Termination(row.Termination),
		FinalFEN:       row.FinalFen,
		CompletedAt:    row.CompletedAt,
	}
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
