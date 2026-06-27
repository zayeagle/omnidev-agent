package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	completionHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true)
	completionBoxStyle    = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#059669")).
				Padding(0, 1)
	completionTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1FAE5"))
)

// CompletionPanelLines renders a pinned completion banner (always visible after success).
func CompletionPanelLines(t *Turn, width int) []string {
	if t == nil || strings.TrimSpace(t.completionMsg) == "" {
		return nil
	}
	if width < 20 {
		width = 60
	}
	boxWidth := width - 4
	if boxWidth < 30 {
		boxWidth = 30
	}
	inner := boxWidth - 4
	if inner < 20 {
		inner = 20
	}

	var rows []string
	for _, line := range strings.Split(strings.TrimSpace(t.completionMsg), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for i, wl := range WrapDisplayWidth(line, inner) {
			if i == 0 {
				rows = append(rows, completionTextStyle.Render(wl))
			} else {
				rows = append(rows, completionTextStyle.Render(wl))
			}
		}
	}
	if t.projectDir != "" && !strings.Contains(t.completionMsg, t.projectDir) {
		rows = append(rows, completionTextStyle.Render("New project path:"))
		for _, wl := range WrapDisplayWidth(t.projectDir, inner) {
			rows = append(rows, completionTextStyle.Render("  "+wl))
		}
	}

	box := completionBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
	return []string{"", completionHeaderStyle.Render("✓ All tasks completed"), box, ""}
}
