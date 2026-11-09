package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ChessLI/internal/dashboard_data"
)

type failingSource struct{}

func (failingSource) Load(context.Context) (dashboard_data.Overview, error) {
	return dashboard_data.Overview{}, errors.New("unavailable")
}

func TestDashboardDisplaysLoadedPlayer(t *testing.T) {
	model := New(dashboard_data.Mock{})

	if !strings.Contains(model.View(), "Loading") {
		t.Fatal("missing loading state")
	}

	cmd := model.Init()
	model, _ = model.Update(cmd())

	for _, want := range []string{
		"Player: Local Player (sample data)",
		"Games: 3  Wins: 2  Draws: 0  Losses: 1",
		"Win vs Alex",
	} {
		if !strings.Contains(model.View(), want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestDashboardDisplaysLoadError(t *testing.T) {
	model := New(failingSource{})
	cmd := model.Init()
	model, _ = model.Update(cmd())

	if !strings.Contains(model.View(), "unavailable") {
		t.Fatal("missing load error")
	}
}

func TestDashboardDisplaysEmptyHistory(t *testing.T) {
	model := New(dashboard_data.Mock{})
	model, _ = model.Update(LoadedMsg{
		Data: dashboard_data.Overview{Username: "New Player"},
	})

	if !strings.Contains(model.View(), "No completed games yet.") {
		t.Fatal("missing empty history state")
	}
}
