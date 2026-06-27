package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	footerModelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	footerSepStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	footerPctStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// FooterBar renders: model · context% · optional hints
func FooterBar(modelName string, contextPct float64, scrollHint, extra string) string {
	pctStr := formatContextPct(contextPct)
	line := footerModelStyle.Render(modelName) +
		footerSepStyle.Render(" · ") +
		footerPctStyle.Render(pctStr)
	if scrollHint != "" {
		line += footerSepStyle.Render(" · ") + footerPctStyle.Render(scrollHint)
	}
	if extra != "" {
		line += footerSepStyle.Render(" · ") + footerPctStyle.Render(extra)
	}
	return line
}

func formatContextPct(pct float64) string {
	if pct < 1 && pct > 0 {
		return fmt.Sprintf("%.1f%%", pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}
