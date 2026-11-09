package play

import (
	"fmt"
	"strings"

	"ChessLI/internal/websocket/protocol"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) View(width int, ready bool) string {
	heading := "Play Chess — /match, /create or /join GAME_ID"
	if m.gameID != "" {
		heading = fmt.Sprintf("Game: %s · You: %s", m.gameID, m.color)
	}
	status := m.message
	if status == "" {
		switch {
		case !ready:
			status = "Disconnected. Restart with the same profile ID to recover."
		case m.action == actionUnknown:
			status = "Outcome unknown. Restart with the same profile ID to recover."
		case m.action == actionQueued:
			status = "Waiting for a matchmaking opponent…"
		case m.action == actionInitializing:
			status = "Waiting for the initial game state…"
		case m.action == actionAwaiting:
			status = "Awaiting server response…"
		case m.action == actionRecoveryRequired:
			status = "Restart with the same profile ID to recover."
		case m.gameID == "":
			status = "/match: find an opponent · /create: new game · /join GAME_ID: join a friend"
		case m.gameID != "" && m.state == nil:
			status = "Waiting for the other player to join."
		case m.state.Status == protocol.GameStatusFinished:
			status = "Game finished. /match, /create or /join GAME_ID"
		case m.turn() != m.color:
			status = "Opponent’s turn · /resign"
		default:
			status = "Your turn · e4, Nf3 or e2e4 · /resign"
		}
	}
	input := m.input
	if lipgloss.Width(input) > width-3 {
		input = ansi.TruncateLeft(input, lipgloss.Width(input)-max(0, width-4), "…")
	}
	content := heading + "\n> " + input + "▏\n" + status
	if m.state == nil {
		return content
	}

	board := m.board()
	details := playerView("White", m.state.White) + "\n" + playerView("Black", m.state.Black)
	if m.state.Outcome != nil {
		details += fmt.Sprintf("\nResult: %s\nReason: %s", m.state.Outcome.Result, m.state.Outcome.Reason)
	}
	details += "\n\nRecent moves (this session):"
	if len(m.moves) == 0 {
		details += "\nNone received yet."
	} else {
		details += "\n" + strings.Join(m.moves, " · ")
	}
	details += "\nClocks shown at last server update."
	details = lipgloss.NewStyle().Width(max(1, width-lipgloss.Width(board)-2)).Render(details)
	return content + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, board+"  ", details)
}
