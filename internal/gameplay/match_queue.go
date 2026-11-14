package gameplay

import (
	"ChessLI/internal/identity"
	"context"
	"log/slog"
	"time"
)

// matchQueue is owned by GameService; every access requires the service mutex.
type matchQueue map[timeControlKey]*MatchmakingEntry

// Admission has already removed canceled players and rejected duplicate profiles.
func (q matchQueue) findOpponent(player *MatchmakingEntry) *MatchmakingEntry {
	key := timeControlKey{initial: player.command.Initial, increment: player.command.Increment}
	waiting, exists := q[key]
	if !exists {
		q[key] = player
		return nil
	}

	delete(q, key)
	return waiting
}

func (q matchQueue) containsProfile(profileID identity.ProfileID) bool {
	for _, waiting := range q {
		if waiting.command.ProfileID == profileID {
			return true
		}
	}

	return false
}

func (q matchQueue) removeCanceledPlayers() {
	for _, waiting := range q {
		if waiting.ctx.Err() == nil {
			continue
		}
		q.remove(waiting)
	}
}

func (q matchQueue) remove(player *MatchmakingEntry) {
	if player == nil {
		return
	}
	key := timeControlKey{initial: player.command.Initial, increment: player.command.Increment}
	// A late cancellation must not remove a replacement entry in the same pool.
	if q[key] != player {
		return
	}
	delete(q, key)
	slog.Info("matchmaking canceled", "event", "matchmaking.canceled", "profile_id", player.command.ProfileID)
}

// MatchmakingEntry is an opaque handle to one admission, created by EnterMatchmaking.
// Keep it to correlate a match or cancel that entry without affecting a newer one.
type MatchmakingEntry struct {
	command EnterMatchmakingCommand
	ctx     context.Context
}

// timeControlKey identifies one matchmaking pool by its clock settings.
type timeControlKey struct {
	initial   time.Duration
	increment time.Duration
}
