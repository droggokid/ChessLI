package identity

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewIdentifiers(t *testing.T) {
	t.Parallel()

	t.Run("profile", func(t *testing.T) {
		t.Parallel()

		first := NewProfileID()
		second := NewProfileID()
		if first == "" || second == "" {
			t.Fatal("NewProfileID() returned an empty identifier")
		}
		if first == second {
			t.Fatalf("NewProfileID() returned duplicate identifier %q", first)
		}
	})

	t.Run("game", func(t *testing.T) {
		t.Parallel()

		first := NewGameID()
		second := NewGameID()
		if first == "" || second == "" {
			t.Fatal("NewGameID() returned an empty identifier")
		}
		if first == second {
			t.Fatalf("NewGameID() returned duplicate identifier %q", first)
		}
	})
}

func TestParseProfile(t *testing.T) {
	t.Parallel()

	id := uuid.NewString()
	profile, err := ParseProfile(id)
	if err != nil {
		t.Fatalf("ParseProfile() error = %v", err)
	}
	if profile.ID != ProfileID(id) {
		t.Fatalf("ParseProfile() ID = %q, want %q", profile.ID, id)
	}

	if _, err := ParseProfile("not-a-uuid"); err == nil {
		t.Fatal("ParseProfile() accepted an invalid profile ID")
	}
}
