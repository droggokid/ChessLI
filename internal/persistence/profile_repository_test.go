package persistence

import (
	"context"
	"testing"

	"ChessLI/internal/identity"
)

func TestProfileRepositoryRejectsInvalidProfileID(t *testing.T) {
	repository := NewProfileRepository(nil)

	if _, err := repository.GetProfileByID(context.Background(), identity.ProfileID("invalid")); err == nil {
		t.Fatal("GetProfileByID() error = nil, want invalid profile ID error")
	}
}
