package dashboard

import (
	"fmt"
	"strings"

	"ChessLI/internal/dashboard_data"
)

func overviewView(data dashboard_data.Overview) string {
	stats := data.Statistics
	total := stats.Wins + stats.Draws + stats.Losses

	var content strings.Builder
	fmt.Fprintf(&content,
		"Dashboard\n\nPlayer: %s\nGames: %d  Wins: %d  Draws: %d  Losses: %d\n\n"+
			"Recent games\n",
		data.Username, total, stats.Wins, stats.Draws, stats.Losses,
	)

	if len(data.Recent) == 0 {
		content.WriteString("No completed games yet.\n")
	} else {
		for _, game := range data.Recent[:min(3, len(data.Recent))] {
			fmt.Fprintf(&content, "%s  %s vs %s (%s)\n",
				game.CompletedAt.Format("2006-01-02"),
				game.Result,
				game.Opponent,
				game.Termination,
			)
		}
	}

	content.WriteString("\nEnter: open Play Chess  r: refresh")
	return content.String()
}
