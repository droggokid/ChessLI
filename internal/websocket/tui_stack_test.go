package websocket

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"ChessLI/internal/dashboard_data"
	"ChessLI/internal/tui"

	tea "charm.land/bubbletea/v2"
)

func TestTUIStartsWithSuppliedProfileID(t *testing.T) {
	server, address := startStackServer(t)
	t.Cleanup(func() { shutdownStackServer(t, server) })
	const profileID = "12345678-1234-1234-1234-123456789012"
	client := startTUI(t, address+"?profileId="+profileID)
	if got := profileFromView(t, client.await(t, "Connected · Profile:")); got != profileID {
		t.Fatalf("startup profile ID = %q, want %q", got, profileID)
	}
	client.program.Send(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	client.await(t, "Connecting to")
	if got := profileFromView(t, client.await(t, "Connected · Profile:")); got != profileID {
		t.Fatalf("reconnected profile ID = %q, want %q", got, profileID)
	}
}

func TestTUITwoClientsCreateJoinMoveReconnectAndResign(t *testing.T) {
	server, address := startStackServer(t)
	t.Cleanup(func() { shutdownStackServer(t, server) })
	white := startTUI(t, address)
	black := startTUI(t, address)
	whiteProfile := profileFromView(t, white.await(t, "Connected · Profile:"))
	black.await(t, "Connected · Profile:")
	white.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	black.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	white.enter("/create")
	waiting := white.await(t, "Waiting for the other player to join.")
	gameID := strings.Fields(strings.Split(waiting, "Game: ")[1])[0]
	black.enter("/join " + gameID)
	black.await(t, "Opponent’s turn")
	white.await(t, "Your turn")
	white.enter("e4")
	white.await(t, "v1 e4")
	black.await(t, "v1 e4")

	// Keep receiving while History is active, then reveal the updated Play view.
	white.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	white.await(t, "Game history is not available yet.")
	black.enter("e7e5")
	black.await(t, "v2 e5")
	white.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	white.await(t, "v2 e5")

	white.program.Send(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	white.await(t, "Connecting to")
	white.await(t, "Awaiting server response…")
	resumed := white.awaitMatching(t, func(view string) bool {
		return strings.Contains(view, "Connected · Profile: "+whiteProfile) &&
			strings.Contains(view, "v2 e5") && strings.Contains(view, "Your turn")
	})
	if !strings.Contains(resumed, "Game: "+gameID) {
		t.Fatal("reconnect changed the game")
	}

	white.enter("Qa5")
	white.await(t, "illegal_move")
	white.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	white.enter("Nf3")
	white.await(t, "v3 Nf3")
	black.await(t, "v3 Nf3")
	black.enter("/resign")
	white.await(t, "Result: white_win")
	black.await(t, "Reason: resignation")
	white.enter("/create")
	white.await(t, "Waiting for the other player to join.")

	for _, client := range []*tuiSession{white, black} {
		client.program.Send(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
		select {
		case err := <-client.done:
			if err != nil {
				t.Fatal("TUI quit:", err)
			}
			client.stopped = true
		case <-client.ctx.Done():
			t.Fatal("TUI did not shut down")
		}
	}
}

func TestTUITwoClientsMatchMoveReconnectAndResign(t *testing.T) {
	server, address := startStackServer(t)
	t.Cleanup(func() { shutdownStackServer(t, server) })
	first, second := startTUI(t, address), startTUI(t, address)
	first.await(t, "Connected · Profile:")
	second.await(t, "Connected · Profile:")
	first.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	second.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	first.enter("/match")
	first.await(t, "Waiting for a matchmaking opponent")
	first.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	first.await(t, "Game history is not available yet.")
	second.enter("/match")
	secondView := second.awaitMatching(t, func(view string) bool {
		return strings.Contains(view, "White: online") && strings.Contains(view, "Black: online")
	})
	first.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	firstView := first.awaitMatching(t, func(view string) bool {
		return strings.Contains(view, "White: online") && strings.Contains(view, "Black: online")
	})
	gameID := strings.Fields(strings.Split(firstView, "Game: ")[1])[0]
	if !strings.Contains(secondView, "Game: "+gameID) {
		t.Fatal("matchmaking assigned different games")
	}
	white, black := first, second
	if strings.Contains(firstView, "You: black") {
		white, black = second, first
	}
	if strings.Contains(firstView, "You: white") == strings.Contains(secondView, "You: white") {
		t.Fatal("matchmaking did not assign opposite colors")
	}
	white.enter("e4")
	white.await(t, "v1 e4")
	black.await(t, "v1 e4")
	white.program.Send(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	white.await(t, "Connecting to")
	resumed := white.awaitMatching(t, func(view string) bool {
		return strings.Contains(view, "Connected · Profile:") && strings.Contains(view, "Opponent’s turn")
	})
	if !strings.Contains(resumed, "Game: "+gameID) {
		t.Fatal("matchmaking recovery changed game")
	}
	black.enter("e5")
	white.await(t, "v2 e5")
	black.await(t, "v2 e5")
	black.enter("/resign")
	white.await(t, "Result: white_win")
	black.await(t, "Reason: resignation")
}

type observedTUI struct {
	tea.Model
	views chan<- string
	ctx   context.Context
}

func (m observedTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	select {
	case m.views <- m.Model.View().Content:
	case <-m.ctx.Done():
	}
	return m, cmd
}

type tuiSession struct {
	program *tea.Program
	views   <-chan string
	ctx     context.Context
	done    <-chan error
	stopped bool
}

func startTUI(t *testing.T, address string) *tuiSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	views := make(chan string, 128)
	model := tui.New(dashboard_data.Mock{}, address)
	program := tea.NewProgram(observedTUI{Model: model, views: views, ctx: ctx},
		tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(), tea.WithoutRenderer(), tea.WithWindowSize(120, 40))
	done := make(chan error, 1)
	session := &tuiSession{program: program, views: views, ctx: ctx, done: done}
	go func() {
		_, err := program.Run()
		if closeErr := model.Close(); err == nil {
			err = closeErr
		}
		done <- err
	}()
	t.Cleanup(func() {
		cancel()
		program.Kill()
		if err := model.Close(); err != nil {
			t.Error("close TUI:", err)
		}
		if !session.stopped {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("TUI cleanup timed out")
			}
		}
	})
	return session
}

func (s *tuiSession) enter(text string) {
	s.program.Send(tea.PasteMsg{Content: text})
	s.program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
}

func (s *tuiSession) await(t *testing.T, text string) string {
	t.Helper()
	return s.awaitMatching(t, func(view string) bool { return strings.Contains(view, text) })
}

func (s *tuiSession) awaitMatching(t *testing.T, matches func(string) bool) string {
	t.Helper()
	var last string
	for {
		select {
		case last = <-s.views:
			if matches(last) {
				return last
			}
		case <-s.ctx.Done():
			t.Fatalf("TUI did not reach expected state. Last view:\n%s", last)
		}
	}
}

func profileFromView(t *testing.T, view string) string {
	t.Helper()
	_, rest, ok := strings.Cut(view, "Connected · Profile: ")
	if !ok || len(strings.Fields(rest)) == 0 {
		t.Fatal("missing profile ID")
	}
	return strings.Fields(rest)[0]
}
