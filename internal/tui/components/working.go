package components

import "github.com/charmbracelet/lipgloss"

var (
	workingSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	workingLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// WorkingIndicator renders the contextual working line above the input (Cursor-style).
func WorkingIndicator(frame int, label string) string {
	if label == "" {
		label = "Working"
	}
	f := spinnerFrames[frame%len(spinnerFrames)]
	return workingSpinnerStyle.Render(f) + " " + workingLabelStyle.Render(label)
}
