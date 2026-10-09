package profile

import (
	"context"
	"testing"

	"ChessLI/internal/identity"
)

func TestProfileRepositoryRejectsInvalidProfileID(t *testing.T) {
	repository := &ProfileRepository{}

	if _, err := repository.GetProfileByID(context.Background(), identity.ProfileID("invalid")); err == nil {
		t.Fatal("GetProfileByID() error = nil, want invalid profile ID error")
	}
}
