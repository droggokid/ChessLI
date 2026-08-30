package gameplay

import (
	"time"

	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

type GameSnapshot struct {
	GameID         identity.GameID
	FEN            string
	Version        uint64
	WhiteProfileID identity.ProfileID
	BlackProfileID identity.ProfileID
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
	LastMoveSAN    string
	Outcome        chess.Outcome
	Termination    TerminationReason
}
