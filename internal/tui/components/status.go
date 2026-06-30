package components

import "github.com/charmbracelet/lipgloss"

// StatusLabel renders a colored status badge.
type StatusLabel struct {
	style lipgloss.Style
}

// NewStatusLabel creates a status label with the appropriate color.
func NewStatusLabel(state string) string {
	s := lipgloss.NewStyle()
	switch state {
	case "Idle":
		s = s.Foreground(lipgloss.Color("#6B7280"))
	case "Thinking":
		s = s.Foreground(lipgloss.Color("#A78BFA")).Italic(true)
	case "Executing", "Working":
		s = s.Foreground(lipgloss.Color("#FBBF24"))
	case "WaitingApproval":
		s = s.Foreground(lipgloss.Color("#F87171")).Bold(true)
	case "Done":
		s = s.Foreground(lipgloss.Color("#34D399"))
	case "Error":
		s = s.Foreground(lipgloss.Color("#F87171")).Bold(true)
	default:
		s = s.Foreground(lipgloss.Color("#6B7280"))
	}
	return s.Render("[" + state + "]")
}
