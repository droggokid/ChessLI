package identity

import (
	"fmt"

	"github.com/google/uuid"
)

type Profile struct {
	ID ProfileID
}

// NewProfile returns a profile with a new unique ID.
func NewProfile() Profile {
	return Profile{ID: NewProfileID()}
}

// ParseProfile parses a UUID into a profile with a canonical ID.
func ParseProfile(value string) (Profile, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return Profile{}, fmt.Errorf("parse profile ID: %w", err)
	}

	return Profile{ID: ProfileID(id.String())}, nil
}
