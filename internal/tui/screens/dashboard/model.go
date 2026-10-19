package dashboard

import (
	"context"
	"fmt"
	"time"

	"ChessLI/internal/dashboard_data"

	tea "charm.land/bubbletea/v2"
)

type Source interface {
	Load(context.Context) (dashboard_data.Overview, error)
}

type StartGameMsg struct{}

type LoadedMsg struct {
	Data dashboard_data.Overview
	Err  error
}

type Model struct {
	source  Source
	data    dashboard_data.Overview
	loading bool
	err     error
}

func New(source Source) Model {
	return Model{
		source:  source,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	source := m.source

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data, err := source.Load(ctx)
		return LoadedMsg{Data: data, Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.data = msg.Data
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return m, func() tea.Msg {
				return StartGameMsg{}
			}

		case "r":
			if !m.loading {
				m.loading = true
				m.err = nil
				return m, m.Init()
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.loading {
		return "Dashboard\n\nLoading player information..."
	}
	if m.err != nil {
		return fmt.Sprintf(
			"Dashboard\n\nCould not load player information:\n%v\n\n"+
				"r: retry  Enter: open Play Chess",
			m.err,
		)
	}

	return overviewView(m.data)
}
