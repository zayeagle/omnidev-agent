package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

const footerExitHint = "Exit: type quit or exit · Ctrl+C"

// FooterExitHint renders a persistent hint for leaving the agent.
func FooterExitHint(width int) string {
	if width < 20 {
		width = 80
	}
	lines := WrapDisplayWidth(footerExitHint, width)
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = footerStyle.Render(line)
	}
	return strings.Join(out, "\n")
}

// FooterBar renders model · context% · hints, wrapped to terminal width.
func FooterBar(width int, modelName string, contextPct float64, scrollHint, extra string) string {
	if width < 20 {
		width = 80
	}
	parts := []string{modelName, formatContextPct(contextPct)}
	if scrollHint != "" {
		parts = append(parts, scrollHint)
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	plain := strings.Join(parts, " · ")
	lines := WrapDisplayWidth(plain, width)
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = footerStyle.Render(line)
	}
	return strings.Join(out, "\n")
}

func formatContextPct(pct float64) string {
	if pct < 1 && pct > 0 {
		return fmt.Sprintf("%.1f%%", pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}
