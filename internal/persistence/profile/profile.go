package profile

import (
	"ChessLI/internal/identity"
	"time"
)

type Profile struct {
	ID        identity.ProfileID
	Username  string
	Email     string
	CreatedAt time.Time
}

type LoginCredential struct {
	ProfileID    identity.ProfileID
	PasswordHash string
}
