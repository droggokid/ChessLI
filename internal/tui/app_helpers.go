package tui

import "strings"

func (m Model) navigation() string {
	names := []string{"Dashboard", "Play Chess", "Game History"}
	tabs := make([]string, len(names))

	for i, name := range names {
		style := m.theme.inactive
		if screen(i) == m.active {
			style = m.theme.active
		}
		tabs[i] = style.Render(name)
	}

	return strings.Join(tabs, " | ")
}

func (m Model) screenView() string {
	switch m.active {
	case playScreen:
		return m.play.View(m.width-m.theme.panel.GetHorizontalFrameSize(), m.connectionStatus == ready)
	case historyScreen:
		return m.history.View()
	default:
		return m.dashboard.View()
	}
}

func (m Model) help() string {
	if m.active == playScreen {
		return "Ctrl+C: quit  Tab: sections  Esc: clear/back  Ctrl+R: reconnect"
	}
	return "Tab: sections  Esc: back  Ctrl+R: reconnect  q/Ctrl+C: quit"
}

func (m Model) connectionView() string {
	status := "Disconnected · Ctrl+R: reconnect"
	switch {
	case m.connectionStatus == dialing:
		status = "Connecting to " + m.network.address + "…"
	case m.connectionStatus == ready:
		status = "Connected · Profile: " + string(m.profileID)
	case m.connectionStatus == awaitingReady:
		status = "Connected · waiting for profile ID…"
	}
	if m.connectionStatus != ready && m.profileID != "" {
		status += " · Profile: " + string(m.profileID)
	}
	if m.networkErr != nil {
		status += "\n" + m.networkErr.Error()
	}
	return status
}
