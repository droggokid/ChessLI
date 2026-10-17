package identity

import (
	"fmt"
	"github.com/google/uuid"
)

type ProfileID string
type GameID string
type DrawOfferID string

// NewProfileID returns a new globally unique profile identifier.
func NewProfileID() ProfileID {
	return ProfileID(uuid.NewString())
}

// ParseProfileID validates a UUID and returns its canonical representation.
func ParseProfileID(value string) (ProfileID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse profile ID: %w", err)
	}
	return ProfileID(id.String()), nil
}

// NewGameID returns a new globally unique game identifier.
func NewGameID() GameID {
	return GameID(uuid.NewString())
}

// NewDrawOfferID returns a new globally unique draw offer identifier.
func NewDrawOfferID() DrawOfferID {
	return DrawOfferID(uuid.NewString())
}
