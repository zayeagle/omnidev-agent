package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	completionHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true)
	completionHeaderWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Bold(true)
	completionSectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA")).Bold(true)
	completionBoxStyle     = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#059669")).
				Padding(0, 1)
	completionWarnBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#B45309")).
				Padding(0, 1)
	completionTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1FAE5"))
)

// CompletionPanel is the pinned post-task report below the transcript.
type CompletionPanel struct {
	Lines []string
}

// CompletionPanelLayout builds the full completion report (no collapse).
func CompletionPanelLayout(t *Turn, width int) CompletionPanel {
	out := CompletionPanel{}
	if t == nil || t.IsChatMode() {
		return out
	}
	if !t.HasCompletion() && len(t.Tasks) == 0 {
		return out
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

	var lines []string
	if t.completionFailed {
		lines = append(lines, completionHeaderWarn.Render("⚠ Task incomplete"))
	} else {
		lines = append(lines, completionHeaderStyle.Render("✓ Task completed"))
	}
	lines = append(lines, "")

	if len(t.Tasks) > 0 {
		lines = append(lines, completionSectionStyle.Render("Completed sub-tasks"))
		tasks := append([]*TaskEntry(nil), t.Tasks...)
		SortTasksByID(tasks)
		if box := RenderTodoListBox(tasks, width); box != "" {
			lines = append(lines, box)
		}
		lines = append(lines, "")
	}

	if detail := strings.TrimSpace(t.acceptanceDetail); detail != "" {
		lines = append(lines, completionSectionStyle.Render("Acceptance results"))
		lines = append(lines, renderPlainBlock(detail, inner, completionTextStyle)...)
		lines = append(lines, "")
	}

	if reason := strings.TrimSpace(t.failureReason); reason != "" {
		lines = append(lines, completionSectionStyle.Render("Failure reason"))
		lines = append(lines, renderPlainBlock(reason, inner, completionTextStyle)...)
		lines = append(lines, "")
	}

	if summary := strings.TrimSpace(t.completionMsg); summary != "" {
		lines = append(lines, completionSectionStyle.Render("Summary"))
		boxStyle := completionBoxStyle
		if t.completionFailed {
			boxStyle = completionWarnBoxStyle
		}
		rows := renderPlainBlock(summary, inner, completionTextStyle)
		if len(rows) > 0 {
			lines = append(lines, boxStyle.Width(boxWidth).Render(strings.Join(rows, "\n")))
		}
		lines = append(lines, "")
	}

	if t.projectDir != "" {
		lines = append(lines, completionSectionStyle.Render("Project location"))
		for _, wl := range WrapDisplayWidth(t.projectDir, inner) {
			lines = append(lines, completionTextStyle.Render("  "+wl))
		}
		lines = append(lines, "")
	}

	out.Lines = lines
	return out
}

func renderPlainBlock(text string, inner int, style lipgloss.Style) []string {
	var rows []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			rows = append(rows, "")
			continue
		}
		for _, wl := range WrapDisplayWidth(line, inner) {
			rows = append(rows, style.Render(wl))
		}
	}
	return rows
}

func completionHeaderText(t *Turn) string {
	if t != nil && t.completionFailed {
		return "⚠ Task incomplete"
	}
	return "✓ Task completed"
}

// CompletionPanelLines is a convenience wrapper returning only line strings.
func CompletionPanelLines(t *Turn, width int) []string {
	return CompletionPanelLayout(t, width).Lines
}

func formatAcceptanceHeader(passed bool, passedN, totalN int) string {
	if totalN <= 0 {
		return "Acceptance results"
	}
	status := "passed"
	if !passed {
		status = "failed"
	}
	return fmt.Sprintf("Acceptance %s %d/%d", status, passedN, totalN)
}
