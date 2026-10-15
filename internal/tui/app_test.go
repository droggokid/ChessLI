package tui

import (
	"ChessLI/internal/dashboard_data"
	"ChessLI/internal/tui/screens/play"
	"ChessLI/internal/websocket/protocol"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestNavigation(t *testing.T) {
	tests := []struct {
		name string
		from screen
		key  tea.Key
		want screen
	}{
		{
			name: "next section",
			from: dashboardScreen,
			key:  tea.Key{Code: tea.KeyTab},
			want: playScreen,
		},
		{
			name: "wrap forward",
			from: historyScreen,
			key:  tea.Key{Code: tea.KeyTab},
			want: dashboardScreen,
		},
		{
			name: "wrap backward",
			from: dashboardScreen,
			key:  tea.Key{Code: tea.KeyTab, Mod: tea.ModShift},
			want: historyScreen,
		},
		{
			name: "escape returns home",
			from: playScreen,
			key:  tea.Key{Code: tea.KeyEscape},
			want: dashboardScreen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
			model.active = tt.from

			next, _ := model.Update(tea.KeyPressMsg(tt.key))
			if got := next.(Model).active; got != tt.want {
				t.Fatalf("active screen = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDashboardEnterOpensPlay(t *testing.T) {
	next, cmd := New(dashboard_data.Mock{}, "ws://localhost:8080/ws").Update(
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
	)
	if cmd == nil {
		t.Fatal("Enter did not produce a command")
	}

	next, _ = next.Update(cmd())
	if got := next.(Model).active; got != playScreen {
		t.Fatalf("active screen = %v, want Play Chess", got)
	}
}

func TestQuitShortcuts(t *testing.T) {
	tests := []struct {
		name string
		from screen
		key  tea.Key
		quit bool
	}{
		{
			name: "q quits dashboard",
			from: dashboardScreen,
			key:  tea.Key{Code: 'q', Text: "q"},
			quit: true,
		},
		{
			name: "q does not quit play",
			from: playScreen,
			key:  tea.Key{Code: 'q', Text: "q"},
		},
		{
			name: "control c quits play",
			from: playScreen,
			key:  tea.Key{Code: 'c', Mod: tea.ModCtrl},
			quit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
			model.active = tt.from

			_, cmd := model.Update(tea.KeyPressMsg(tt.key))
			if !tt.quit {
				if cmd != nil {
					t.Fatal("unexpected quit command")
				}
				return
			}

			if cmd == nil {
				t.Fatal("missing quit command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("command did not request quit")
			}
		})
	}
}

func TestResizeKeepsViewWithinTerminal(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 0, Height: 0},
		{Width: 1, Height: 1},
		{Width: 40, Height: 10},
		{Width: 60, Height: 16},
		{Width: 120, Height: 40},
	}

	for _, size := range sizes {
		next, _ := New(dashboard_data.Mock{}, "ws://localhost:8080/ws").Update(size)
		model := next.(Model)
		view := model.View()

		if model.width != size.Width || model.height != size.Height {
			t.Fatalf("dimensions = %dx%d, want %dx%d",
				model.width, model.height, size.Width, size.Height)
		}
		if !view.AltScreen {
			t.Fatal("alternate screen disabled after resize")
		}
		if view.Content != "" && (lipgloss.Width(view.Content) > size.Width || lipgloss.Height(view.Content) > size.Height) {
			t.Fatalf("view is %dx%d in %dx%d terminal", lipgloss.Width(view.Content), lipgloss.Height(view.Content), size.Width, size.Height)
		}
	}
}

func TestDashboardLoadsWhileAnotherScreenIsActive(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	model.active = historyScreen

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("missing dashboard load command")
	}

	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatal("expected dashboard loading and connection commands")
	}
	next, _ := model.Update(batch[0]())
	got := next.(Model)

	if got.active != historyScreen {
		t.Fatal("dashboard loading changed the active screen")
	}
	if !strings.Contains(got.dashboard.View(), "Player: Local Player") {
		t.Fatal("dashboard did not receive its loaded data")
	}
}

func TestPlayInputReceivesTextAndEscapeBeforeNavigation(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	defer model.Close()
	model.active = playScreen
	next, _ := model.Update(tea.PasteMsg{Content: "/join game-"})
	next, cmd := next.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	if cmd != nil || !strings.Contains(next.(Model).play.View(76, true), "/join game-q") {
		t.Fatal("root swallowed text input")
	}
	next, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if next.(Model).active != playScreen || next.(Model).play.HasInput() {
		t.Fatal("escape did not clear input before navigating")
	}
	next, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if next.(Model).active != dashboardScreen {
		t.Fatal("second escape did not navigate")
	}
}

func TestRootRoutesStateAndErrorsWhilePlayIsInactive(t *testing.T) {
	for _, active := range []screen{dashboardScreen, historyScreen} {
		model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
		t.Cleanup(func() { _ = model.Close() })
		model.active = active
		model.connectionStatus = ready
		model.play, _ = model.play.Update(tea.PasteMsg{Content: "/create"}, true)
		var requestCmd tea.Cmd
		model.play, requestCmd = model.play.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), true)
		request := requestCmd().(play.RequestMsg)
		next, cmd := model.Update(receivedMsg{generation: model.generation, event: protocol.ServerEvent{
			Type: protocol.ServerGameCreated, RequestID: request.RequestID,
			Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorWhite},
		}})
		if cmd == nil {
			t.Fatal("receive was not rearmed after assignment")
		}
		model = next.(Model)
		next, cmd = model.Update(receivedMsg{generation: model.generation, event: protocol.ServerEvent{
			Type:  protocol.ServerGameInitial,
			State: &protocol.GameStatePayload{GameID: "game-1", FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", Status: protocol.GameStatusActive},
		}})
		model = next.(Model)
		if cmd == nil || model.active != active || !strings.Contains(model.play.View(76, true), "Your turn") {
			t.Fatal("background initial state was not routed")
		}
		next, cmd = model.Update(receivedMsg{generation: model.generation, event: protocol.ServerEvent{
			Type: protocol.ServerError, Error: &protocol.ErrorPayload{Code: protocol.ErrorIllegalMove, Message: "illegal move"},
		}})
		if cmd == nil || !strings.Contains(next.(Model).play.View(76, true), "illegal_move") {
			t.Fatal("background application error was not routed")
		}
	}
}

func TestRootRoutesMatchmakingWhilePlayIsInactive(t *testing.T) {
	for _, active := range []screen{dashboardScreen, historyScreen} {
		model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
		t.Cleanup(func() { _ = model.Close() })
		model.active, model.connectionStatus = active, ready
		model.play, _ = model.play.Update(tea.PasteMsg{Content: "/match"}, true)
		var cmd tea.Cmd
		model.play, cmd = model.play.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), true)
		request := cmd().(play.RequestMsg)
		state := &protocol.GameStatePayload{GameID: "game-1", FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", Status: protocol.GameStatusActive}
		presence := *state
		presence.Black = &protocol.PlayerState{Color: protocol.ColorBlack, Connected: true}
		for _, step := range []struct {
			event protocol.ServerEvent
			want  string
		}{
			{protocol.ServerEvent{Type: protocol.ServerMatchmakingEntered, RequestID: request.RequestID,
				Matchmaking: &protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0}}, "Waiting for a matchmaking opponent"},
			{protocol.ServerEvent{Type: protocol.ServerMatchFound, RequestID: request.RequestID,
				Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorWhite}}, "Waiting for the initial game state"},
			{protocol.ServerEvent{Type: protocol.ServerGameInitial, State: state}, "Your turn"},
			{protocol.ServerEvent{Type: protocol.ServerGameState, State: &presence}, "Black: online"},
		} {
			next, receive := model.Update(receivedMsg{generation: model.generation, event: step.event})
			model = next.(Model)
			if receive == nil || model.active != active || !strings.Contains(model.play.View(76, true), step.want) {
				t.Fatalf("background %s was not routed: %s", step.event.Type, model.play.View(76, true))
			}
			next, _ = model.Update(play.TimeoutMsg{RequestID: request.RequestID})
			model = next.(Model)
			if !strings.Contains(model.play.View(76, true), step.want) {
				t.Fatal("background deadline changed matchmaking state")
			}
		}
	}
}

func TestRootDistinguishesMessageErrorsFromConnectionLoss(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	defer model.Close()
	model.connectionStatus = ready
	next, cmd := model.Update(receivedMsg{generation: model.generation, err: &messageError{Err: protocol.ErrUnsupportedEvent}})
	if cmd == nil || next.(Model).connectionStatus != ready {
		t.Fatal("recoverable message error stopped reception")
	}
	next, cmd = next.Update(receivedMsg{generation: model.generation, event: protocol.ServerEvent{Type: protocol.ServerGameState}})
	if cmd == nil || next.(Model).networkErr != nil {
		t.Fatal("successful reception did not clear the previous message error")
	}
	next, cmd = next.Update(receivedMsg{generation: model.generation, err: errors.New("connection lost")})
	if cmd != nil || next.(Model).connectionStatus == ready || !strings.Contains(next.(Model).connectionView(), "connection lost") {
		t.Fatal("connection failure was not displayed or stopped")
	}
}

func TestRootWaitsForConnectionReadyBeforeSubmission(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	defer model.Close()
	model.active = playScreen
	if !strings.Contains(model.connectionView(), "Connecting to") {
		t.Fatal("dialing was not displayed")
	}
	next, _ := model.Update(connectedMsg{generation: model.generation})
	model = next.(Model)
	if !strings.Contains(model.connectionView(), "waiting for profile ID") {
		t.Fatal("socket connection was mistaken for readiness")
	}
	next, _ = model.Update(tea.PasteMsg{Content: "/create"})
	next, cmd := next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Fatal("allowed submission before connection.ready")
	}
	model = next.(Model)
	next, _ = model.Update(receivedMsg{generation: model.generation, event: protocol.ServerEvent{
		Type: protocol.ServerConnectionReady, Ready: &protocol.ConnectionReadyPayload{ProfileID: "profile-1"},
	}})
	if !strings.Contains(next.(Model).connectionView(), "Connected · Profile: profile-1") {
		t.Fatal("readiness was not displayed")
	}
	_, cmd = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil || cmd().(play.RequestMsg).Type != protocol.ClientCreateGame {
		t.Fatal("connection.ready did not permit submission")
	}
}

func TestRootIgnoresOldConnectionEvents(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	defer model.Close()
	model.generation = 2
	next, cmd := model.Update(receivedMsg{generation: 1, event: protocol.ServerEvent{Type: protocol.ServerConnectionReady, Ready: &protocol.ConnectionReadyPayload{ProfileID: "old-profile"}}})
	if cmd != nil || next.(Model).profileID != "" {
		t.Fatal("old connection updated current identity or rearmed reception")
	}
}

func TestPlayViewKeepsInputVisibleWithinTerminal(t *testing.T) {
	model := New(dashboard_data.Mock{}, "ws://localhost:8080/ws")
	defer model.Close()
	model.active = playScreen
	model.connectionStatus = ready
	model.play, _ = model.play.Update(tea.PasteMsg{Content: "/create"}, true)
	var cmd tea.Cmd
	model.play, cmd = model.play.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), true)
	request := cmd().(play.RequestMsg)
	model.play, _ = model.play.Update(protocol.ServerEvent{Type: protocol.ServerGameCreated, RequestID: request.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "12345678-1234-1234-1234-123456789012", Color: protocol.ColorWhite}}, true)
	model.play, _ = model.play.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial,
		State: &protocol.GameStatePayload{GameID: "12345678-1234-1234-1234-123456789012", FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", Status: protocol.GameStatusActive}}, true)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 120, Height: 40}} {
		next, _ := model.Update(size)
		view := next.View().Content
		if lipgloss.Width(view) > size.Width || lipgloss.Height(view) > size.Height {
			t.Fatalf("view is %dx%d in %dx%d terminal", lipgloss.Width(view), lipgloss.Height(view), size.Width, size.Height)
		}
		if !strings.Contains(view, "> ") {
			t.Fatal("move input was clipped")
		}
		if !strings.Contains(view, "8♜") || !strings.Contains(view, "1♖") {
			t.Fatalf("board ranks were clipped:\n%s", view)
		}
	}
}
