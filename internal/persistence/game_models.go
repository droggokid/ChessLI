package persistence

import (
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

type CompletedGame struct {
	ID               identity.GameID    `db:"id"`
	WhiteProfileID   identity.ProfileID `db:"white_profile_id"`
	BlackProfileID   identity.ProfileID `db:"black_profile_id"`
	InitialTimeMS    int64              `db:"initial_time_ms"`
	IncrementMS      int64              `db:"increment_ms"`
	Outcome          domain.Outcome     `db:"outcome"`
	Termination      domain.Termination `db:"termination"`
	FinalFEN         string             `db:"final_fen"`
	WhiteRemainingMS int64              `db:"white_remaining_ms"`
	BlackRemainingMS int64              `db:"black_remaining_ms"`
	CompletedAt      time.Time          `db:"completed_at"`
	Moves            []Move             `db:"moves"`
}

type Summary struct {
	ID             identity.GameID    `db:"id"`
	WhiteProfileID identity.ProfileID `db:"white_profile_id"`
	BlackProfileID identity.ProfileID `db:"black_profile_id"`
	InitialTimeMS  int64              `db:"initial_time_ms"`
	IncrementMS    int64              `db:"increment_ms"`
	Outcome        domain.Outcome     `db:"outcome"`
	Termination    domain.Termination `db:"termination"`
	CompletedAt    time.Time          `db:"completed_at"`
}

// Move is the JSONB representation embedded in a completed-game row.
type Move struct {
	Ply              int    `json:"ply"`
	UCI              string `json:"uci"`
	WhiteRemainingMS int64  `json:"white_remaining_ms"`
	BlackRemainingMS int64  `json:"black_remaining_ms"`
}
