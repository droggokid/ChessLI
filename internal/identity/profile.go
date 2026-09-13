package identity

import (
	"fmt"

	"github.com/google/uuid"
)

type Profile struct {
	ID ProfileID
}

func NewProfile() Profile {
	return Profile{ID: NewProfileID()}
}

// ParseProfile builds a profile from a previously issued profile ID.
func ParseProfile(value string) (Profile, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return Profile{}, fmt.Errorf("parse profile ID: %w", err)
	}

	return Profile{ID: ProfileID(id.String())}, nil
}
