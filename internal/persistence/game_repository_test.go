package persistence

import (
	"context"
	"testing"

	"ChessLI/internal/identity"
)

func TestGameRepositoryRejectsInvalidGameID(t *testing.T) {
	repository := NewGameRepository(nil)

	if _, err := repository.GetGameByID(context.Background(), identity.GameID("invalid")); err == nil {
		t.Fatal("GetGameByID() error = nil, want invalid game ID error")
	}
}

func TestGameRepositoryRejectsNonSequentialMovePly(t *testing.T) {
	repository := NewGameRepository(nil)

	err := repository.CreateGame(context.Background(), CompletedGame{
		ID:             identity.GameID("00000000-0000-0000-0000-000000000001"),
		WhiteProfileID: identity.ProfileID("00000000-0000-0000-0000-000000000002"),
		BlackProfileID: identity.ProfileID("00000000-0000-0000-0000-000000000003"),
		Moves: []Move{{
			Ply:              2,
			UCI:              "e2e4",
			WhiteRemainingMS: 60000,
			BlackRemainingMS: 60000,
		}},
	})
	if err == nil {
		t.Fatal("CreateGame() error = nil, want non-sequential ply error")
	}
}
