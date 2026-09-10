package identity

import "testing"

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
