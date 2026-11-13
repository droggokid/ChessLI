package identity

import (
	"testing"
)

func TestParseProfileID(t *testing.T) {
	t.Parallel()

	const id = "a0b1c2d3-e4f5-4678-9012-3456789abcde"
	profileID, err := ParseProfileID("{A0B1C2D3-E4F5-4678-9012-3456789ABCDE}")
	if err != nil {
		t.Fatalf("ParseProfileID() error = %v", err)
	}
	if profileID != ProfileID(id) {
		t.Fatalf("ParseProfileID() = %q, want %q", profileID, id)
	}

	if _, err := ParseProfileID("not-a-uuid"); err == nil {
		t.Fatal("ParseProfileID() accepted an invalid profile ID")
	}
}
