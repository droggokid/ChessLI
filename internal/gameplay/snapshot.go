package gameplay

import (
	"ChessLI/internal/identity"
	"time"

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
	Method         chess.Method
}
