package tui

import "charm.land/lipgloss/v2"

type theme struct {
	title    lipgloss.Style
	active   lipgloss.Style
	inactive lipgloss.Style
	panel    lipgloss.Style
	status   lipgloss.Style
}

func newTheme() theme {
	return theme{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A3E635")),
		active:   lipgloss.NewStyle().Bold(true).Underline(true),
		inactive: lipgloss.NewStyle(),
		panel:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		status:   lipgloss.NewStyle().Faint(true),
	}
}
