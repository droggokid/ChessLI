package persistence

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/profile"
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=profile_repository_mock.go -package=persistence -mock_names=Repository=MockProfileRepository ChessLI/internal/persistence ProfileRepository

type ProfileRepository interface {
	CreateProfile(ctx context.Context, username, email, passwordHash string) (profile.Profile, error)
	GetProfileByID(ctx context.Context, iid identity.ProfileID) (profile.Profile, error)
	GetProfilesByIDs(ctx context.Context, ids []identity.ProfileID) ([]profile.Profile, error)
	GetCredentialByEmail(ctx context.Context, email string) (profile.LoginCredential, error)
}

type ProfileRepositoryImpl struct {
	db   *pgxpool.Pool
	psql sq.StatementBuilderType
}

// NewProfileRepository returns a profile repository backed by db.
func NewProfileRepository(db *pgxpool.Pool) *ProfileRepositoryImpl {
	return &ProfileRepositoryImpl{db: db, psql: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

// CreateProfile stores a new profile with the supplied password hash.
func (r *ProfileRepositoryImpl) CreateProfile(ctx context.Context, username, email, passwordHash string) (profile.Profile, error) {
	query, args, err := r.psql.Insert("profiles").
		Columns("id", "username", "email", "password_hash").
		Values(uuid.New(), username, email, passwordHash).
		Suffix("RETURNING id, username, email, created_at").
		ToSql()
	if err != nil {
		return profile.Profile{}, err
	}

	return scanProfile(r.db.QueryRow(ctx, query, args...))
}

// GetProfileByID returns a profile by ID.
func (r *ProfileRepositoryImpl) GetProfileByID(ctx context.Context, id identity.ProfileID) (profile.Profile, error) {
	profileID, err := uuid.Parse(string(id))
	if err != nil {
		return profile.Profile{}, fmt.Errorf("parse profile ID: %w", err)
	}

	query, args, err := r.psql.Select("id", "username", "email", "created_at").
		From("profiles").Where(sq.Eq{"id": profileID}).ToSql()
	if err != nil {
		return profile.Profile{}, err
	}

	return scanProfile(r.db.QueryRow(ctx, query, args...))
}

// GetProfilesByIDs returns matching profiles ordered by ID, omitting missing IDs.
func (r *ProfileRepositoryImpl) GetProfilesByIDs(ctx context.Context, ids []identity.ProfileID) ([]profile.Profile, error) {
	profileIDs := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		profileID, err := uuid.Parse(string(id))
		if err != nil {
			return nil, fmt.Errorf("parse profile ID: %w", err)
		}
		profileIDs[i] = profileID
	}

	query, args, err := r.psql.Select("id", "username", "email", "created_at").
		From("profiles").Where("id = ANY(?::uuid[])", profileIDs).
		OrderBy("id").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (profile.Profile, error) {
		return scanProfile(row)
	})
}

// GetCredentialByEmail returns login credentials using a case-insensitive email lookup.
func (r *ProfileRepositoryImpl) GetCredentialByEmail(ctx context.Context, email string) (profile.LoginCredential, error) {
	query, args, err := r.psql.Select("id", "password_hash").From("profiles").
		Where("LOWER(email) = LOWER(?)", email).ToSql()
	if err != nil {
		return profile.LoginCredential{}, err
	}

	var credential profile.LoginCredential
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, query, args...).Scan(&id, &credential.PasswordHash); err != nil {
		return profile.LoginCredential{}, err
	}
	credential.ProfileID = identity.ProfileID(id.String())
	return credential, nil
}

func scanProfile(row pgx.Row) (profile.Profile, error) {
	var p profile.Profile
	var id uuid.UUID
	if err := row.Scan(&id, &p.Username, &p.Email, &p.CreatedAt); err != nil {
		return profile.Profile{}, err
	}
	p.ID = identity.ProfileID(id.String())
	return p, nil
}
