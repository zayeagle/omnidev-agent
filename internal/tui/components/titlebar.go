package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Titlebar colors
var (
	titlebarBg  = lipgloss.Color("#1E1B4B")
	titlebarFg  = lipgloss.Color("#7C3AED")
	titlebarDim = lipgloss.Color("#A78BFA")
	titlebarSep = lipgloss.Color("#374151")
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(titlebarFg).Background(titlebarBg).Padding(0, 1)
	sepStyle   = lipgloss.NewStyle().Foreground(titlebarSep)
	statusDim  = lipgloss.NewStyle().Foreground(titlebarDim).Italic(true)
)

// Titlebar renders the top bar with version and status.
func Titlebar(width int, version, status string) string {
	if width < 10 {
		width = 80
	}

	title := titleStyle.Render(" omnidev-agent " + version + " ")
	statusLabel := statusDim.Render("[" + status + "]")

	pad := width - lipgloss.Width(title) - lipgloss.Width(statusLabel) - 3
	if pad < 0 {
		pad = 0
	}

	return fmt.Sprintf("%s %s %s %s\n%s",
		title,
		sepStyle.Render("│"),
		strings.Repeat(" ", pad),
		statusLabel,
		sepStyle.Render(strings.Repeat("─", width)),
	)
}
