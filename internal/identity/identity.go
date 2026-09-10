package identity

import "github.com/google/uuid"

type ProfileID string
type GameID string
type DrawOfferID string

// NewProfileID returns a new globally unique profile identifier.
func NewProfileID() ProfileID {
	return ProfileID(uuid.NewString())
}

// NewGameID returns a new globally unique game identifier.
func NewGameID() GameID {
	return GameID(uuid.NewString())
}

// NewDrawOfferID returns a new globally unique draw offer identifier.
func NewDrawOfferID() DrawOfferID {
	return DrawOfferID(uuid.NewString())
}
