package profile

import (
	"ChessLI/internal/database/sqlc"
	"ChessLI/internal/identity"
	"ChessLI/internal/profile"
	"context"
	"fmt"

	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=repository_mock.go -package=profile -mock_names=Repository=MockProfileRepository ChessLI/internal/persistence/profile Repository

type Repository interface {
	CreateProfile(ctx context.Context, username, email, passwordHash string) (profile.Profile, error)
	GetProfileByID(ctx context.Context, iid identity.ProfileID) (profile.Profile, error)
	GetProfilesByIDs(ctx context.Context, ids []identity.ProfileID) ([]profile.Profile, error)
	GetCredentialByEmail(ctx context.Context, email string) (profile.LoginCredential, error)
}

type ProfileRepository struct {
	queries *sqlc.Queries
}

// NewProfileRepository returns a profile repository backed by queries.
func NewProfileRepository(queries sqlc.Queries) *ProfileRepository {
	return &ProfileRepository{
		queries: &queries,
	}
}

// CreateProfile stores a new profile with the supplied password hash.
func (r *ProfileRepository) CreateProfile(ctx context.Context, username, email, passwordHash string) (profile.Profile, error) {
	row, err := r.queries.CreateProfile(ctx, sqlc.CreateProfileParams{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return profile.Profile{}, err
	}

	return profile.Profile{
		ID:        identity.ProfileID(row.ID.String()),
		Username:  row.Username,
		Email:     row.Email,
		CreatedAt: row.CreatedAt,
	}, nil
}

// GetProfileByID returns a profile by ID.
func (r *ProfileRepository) GetProfileByID(ctx context.Context, id identity.ProfileID) (profile.Profile, error) {
	profileID, err := uuid.Parse(string(id))
	if err != nil {
		return profile.Profile{}, fmt.Errorf("parse profile ID: %w", err)
	}

	row, err := r.queries.GetProfileByID(ctx, profileID)
	if err != nil {
		return profile.Profile{}, err
	}

	return profile.Profile{
		ID:        identity.ProfileID(row.ID.String()),
		Username:  row.Username,
		Email:     row.Email,
		CreatedAt: row.CreatedAt,
	}, nil
}

// GetProfilesByIDs returns matching profiles ordered by ID, omitting missing IDs.
func (r *ProfileRepository) GetProfilesByIDs(ctx context.Context, ids []identity.ProfileID) ([]profile.Profile, error) {
	profileIDs := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		profileID, err := uuid.Parse(string(id))
		if err != nil {
			return nil, fmt.Errorf("parse profile ID: %w", err)
		}
		profileIDs[i] = profileID
	}

	rows, err := r.queries.GetProfilesByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}

	profiles := make([]profile.Profile, len(rows))
	for i, row := range rows {
		profiles[i] = profile.Profile{
			ID:        identity.ProfileID(row.ID.String()),
			Username:  row.Username,
			Email:     row.Email,
			CreatedAt: row.CreatedAt,
		}
	}
	return profiles, nil
}

// GetCredentialByEmail returns login credentials using a case-insensitive email lookup.
func (r *ProfileRepository) GetCredentialByEmail(ctx context.Context, email string) (profile.LoginCredential, error) {
	row, err := r.queries.GetProfileByEmailForLogin(ctx, email)
	if err != nil {
		return profile.LoginCredential{}, err
	}

	return profile.LoginCredential{
		ProfileID:    identity.ProfileID(row.ID.String()),
		PasswordHash: row.PasswordHash,
	}, nil
}
