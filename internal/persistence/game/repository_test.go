package game

import (
	"context"
	"testing"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"
)

func TestGameRepositoryRejectsInvalidGameID(t *testing.T) {
	repository := &GameRepository{}

	if _, err := repository.GetByID(context.Background(), identity.GameID("invalid")); err == nil {
		t.Fatal("GetByID() error = nil, want invalid game ID error")
	}
}

func TestGameRepositoryRejectsNonSequentialMovePly(t *testing.T) {
	repository := &GameRepository{}
	gameID := identity.GameID("00000000-0000-0000-0000-000000000001")

	err := repository.CreateMoves(context.Background(), gameID, []domain.Move{{
		Ply:            2,
		UCI:            "e2e4",
		WhiteRemaining: time.Minute,
		BlackRemaining: time.Minute,
	}})
	if err == nil {
		t.Fatal("CreateMoves() error = nil, want non-sequential ply error")
	}
}
