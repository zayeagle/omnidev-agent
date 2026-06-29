package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	workingSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	workingLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D4D4D8"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// WorkingIndicator renders the contextual working line above the input (Cursor-style).
func WorkingIndicator(frame int, label string, width int) string {
	if label == "" {
		label = "Working"
	}
	if width < 20 {
		width = 80
	}
	f := spinnerFrames[frame%len(spinnerFrames)]
	prefix := workingSpinnerStyle.Render(f) + " "
	prefixWidth := runewidth.StringWidth(f + " ")
	contentWidth := width - prefixWidth
	if contentWidth < 10 {
		contentWidth = 10
	}
	lines := WrapDisplayWidth(label, contentWidth)
	if len(lines) == 0 {
		return prefix + workingLabelStyle.Render(label)
	}
	out := make([]string, len(lines))
	out[0] = prefix + workingLabelStyle.Render(lines[0])
	for i := 1; i < len(lines); i++ {
		out[i] = strings.Repeat(" ", prefixWidth) + workingLabelStyle.Render(lines[i])
	}
	return strings.Join(out, "\n")
}
