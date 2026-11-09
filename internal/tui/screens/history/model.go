package history

type Model struct{}

func (m Model) View() string {
	return "Game History\n\n" +
		"Game history is not available yet.\n\n" +
		"Esc: return to Dashboard"
}
