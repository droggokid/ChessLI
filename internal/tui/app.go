package tui

import (
	"ChessLI/internal/identity"
	"ChessLI/internal/tui/screens/dashboard"
	"ChessLI/internal/tui/screens/history"
	"ChessLI/internal/tui/screens/play"
	"errors"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/coder/websocket"
)

type screen uint8

const (
	dashboardScreen screen = iota
	playScreen
	historyScreen
	screenCount
)

type connectionStatus uint8

const (
	disconnected connectionStatus = iota
	dialing
	awaitingReady
	ready
)

type Model struct {
	active           screen
	width, height    int
	dashboard        dashboard.Model
	play             *play.Model
	history          history.Model
	theme            theme
	network          *connection
	client           *websocket.Conn
	generation       uint64
	connectionStatus connectionStatus
	profileID        identity.ProfileID
	networkErr       error
}

func New(source dashboard.Source, address string) Model {
	return Model{
		width:            80,
		height:           24,
		dashboard:        dashboard.New(source),
		play:             &play.Model{},
		theme:            newTheme(),
		network:          newConnection(address),
		connectionStatus: dialing,
		generation:       1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.dashboard.Init(), m.network.connect(m.profileID, m.generation))
}

// Close must be called after the Bubble Tea program returns, including on errors.
func (m Model) Close() error { return m.network.Close() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectedMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.networkErr = msg.err
		if msg.err != nil {
			m.connectionStatus = disconnected
			return m, nil
		}
		m.client = msg.client
		m.connectionStatus = awaitingReady
		return m, m.network.receive(m.client, m.generation)

	case receivedMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		if msg.err != nil {
			m.networkErr = msg.err
			if _, ok := errors.AsType[*messageError](msg.err); ok {
				return m, m.network.receive(m.client, m.generation)
			}
			m.connectionStatus = disconnected
			m.play, _ = m.play.Update(play.DisconnectedMsg{}, false)
			return m, nil
		}
		m.networkErr = nil
		var cmd tea.Cmd
		if msg.event.Ready != nil {
			m.connectionStatus = ready
			m.profileID = msg.event.Ready.ProfileID
			m.play, cmd = m.play.Recover()
		} else {
			m.play, cmd = m.play.Update(msg.event, m.connectionStatus == ready)
		}
		return m, tea.Batch(m.network.receive(m.client, m.generation), cmd)

	case play.RequestMsg:
		if !m.play.IsPending(msg.RequestID) {
			return m, nil
		}
		if m.connectionStatus != ready || m.client == nil {
			m.play, _ = m.play.Update(play.SendResultMsg{RequestID: msg.RequestID, Err: errors.New("disconnected")}, false)
			return m, nil
		}
		return m, tea.Batch(m.network.send(m.client, m.generation, msg), m.network.responseDeadline(msg.RequestID))

	case sentMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		if msg.result.Err != nil {
			m.networkErr = msg.result.Err
			m.connectionStatus = disconnected
			m.play, _ = m.play.Update(play.DisconnectedMsg{}, false)
		}
		var cmd tea.Cmd
		m.play, cmd = m.play.Update(msg.result, m.connectionStatus == ready)
		return m, cmd

	case play.TimeoutMsg:
		var cmd tea.Cmd
		m.play, cmd = m.play.Update(msg, m.connectionStatus == ready)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = max(msg.Width, 0)
		m.height = max(msg.Height, 0)
		return m, nil

	case dashboard.StartGameMsg:
		m.active = playScreen
		return m, nil

	case dashboard.LoadedMsg:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.active != playScreen {
				return m, tea.Quit
			}

		case "ctrl+r":
			if m.connectionStatus == dialing {
				return m, nil
			}
			m.generation++
			m.connectionStatus = dialing
			m.networkErr = nil
			m.client = nil
			m.play, _ = m.play.Update(play.DisconnectedMsg{}, false)
			return m, m.network.connect(m.profileID, m.generation)

		case "tab":
			m.active = (m.active + 1) % screenCount
			return m, nil

		case "shift+tab":
			m.active = (m.active + screenCount - 1) % screenCount
			return m, nil

		case "esc":
			if m.active == playScreen && m.play.HasInput() {
				var cmd tea.Cmd
				m.play, cmd = m.play.Update(msg, m.connectionStatus == ready)
				return m, cmd
			}
			if m.active != dashboardScreen {
				m.active = dashboardScreen
				return m, nil
			}
		}
	}

	if m.active == playScreen {
		var cmd tea.Cmd
		m.play, cmd = m.play.Update(msg, m.connectionStatus == ready)
		return m, cmd
	}

	if m.active == dashboardScreen {
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() tea.View {
	var content string

	switch {
	case m.width == 0 || m.height == 0:
		content = ""

	case m.width < 80 || m.height < 24:
		content = lipgloss.NewStyle().
			MaxWidth(m.width).
			MaxHeight(m.height).
			Render("Resize to at least 80x24.\nCtrl+C: quit")

	default:
		connection := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(2).Render(m.connectionView())
		content = m.theme.title.Render("ChessLI") + "\n" +
			m.navigation() + "\n" + connection + "\n\n" +
			m.theme.panel.Width(m.width).Render(m.screenView()) + "\n\n" +
			m.theme.status.MaxWidth(m.width).Render(m.help())
	}

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
