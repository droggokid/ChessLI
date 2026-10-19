package play

import (
	"fmt"
	"strings"
	"unicode"

	"ChessLI/internal/websocket/protocol"

	"github.com/corentings/chess/v2"
)

func (m *Model) turn() protocol.Color {
	if m.position != nil && m.position.Turn() == chess.Black {
		return protocol.ColorBlack
	}
	return protocol.ColorWhite
}

func isUCI(move string) bool {
	if len(move) != 4 && len(move) != 5 {
		return false
	}
	for _, offset := range []int{0, 2} {
		if move[offset] < 'a' || move[offset] > 'h' || move[offset+1] < '1' || move[offset+1] > '8' {
			return false
		}
	}
	return len(move) == 4 || strings.ContainsRune("qrbn", rune(move[4]))
}

func cleanInput(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
}

func (m *Model) board() string {
	if m.position == nil {
		return ""
	}
	perspective := chess.White
	if m.color == protocol.ColorBlack {
		perspective = chess.Black
	}
	return strings.Trim(m.position.Board().Draw2(perspective, true), "\n")
}

func playerView(label string, player *protocol.PlayerState) string {
	if player == nil {
		return label + ": not joined"
	}
	presence := "offline"
	if player.Connected {
		presence = "online"
	}
	clock := ""
	if player.RemainingMilliseconds != nil {
		seconds := max(0, *player.RemainingMilliseconds/1000)
		clock = fmt.Sprintf(" · %d:%02d", seconds/60, seconds%60)
	}
	return label + ": " + presence + clock
}
