package main

import (
	"ChessLI/internal/dashboard_data"
	"ChessLI/internal/tui"
	"flag"
	"fmt"
	"net/url"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	profileID := flag.String("profile-id", "", "connect using an existing profile UUID")
	flag.Parse()
	model := tui.New(dashboard_data.Mock{}, "ws://localhost:8080/ws?profileId="+url.QueryEscape(*profileID))
	_, err := tea.NewProgram(model).Run()
	closeErr := model.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ChessLI:", err)
		os.Exit(1)
	}
}
