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
	completionHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true)
)

// CompletionPanelLines renders a pinned completion banner (always visible after success).
func CompletionPanelLines(t *Turn, width int) []string {
	if t == nil || t.IsChatMode() {
		return nil
	}
	if strings.TrimSpace(t.projectDir) == "" && strings.TrimSpace(t.completionMsg) == "" {
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
	if t.projectDir != "" {
		rows = append(rows, completionTextStyle.Render("Project location:"))
		for _, wl := range WrapDisplayWidth(t.projectDir, inner) {
			rows = append(rows, completionTextStyle.Render("  "+wl))
		}
		rows = append(rows, completionHintStyle.Render("  Open this directory in your browser or editor to run the result."))
	} else if msg := strings.TrimSpace(t.completionMsg); msg != "" {
		for _, line := range strings.Split(msg, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			for _, wl := range WrapDisplayWidth(line, inner) {
				rows = append(rows, completionTextStyle.Render(wl))
			}
		}
	}

	if len(rows) == 0 {
		return nil
	}

	box := completionBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
	return []string{"", completionHeaderStyle.Render("✓ Task completed"), box, ""}
}
