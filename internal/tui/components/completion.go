package components

import (
	"fmt"
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
	completionToggleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
)

// CompletionPanel renders the pinned completion banner and optional collapsible tasks.
// tasksToggleLine is the 0-based line index within Lines for the click-to-expand row, or -1.
type CompletionPanel struct {
	Lines           []string
	TasksToggleLine int
}

// CompletionPanelLayout builds completion UI below the transcript.
func CompletionPanelLayout(t *Turn, width int) CompletionPanel {
	out := CompletionPanel{TasksToggleLine: -1}
	if t == nil || t.IsChatMode() {
		return out
	}
	if strings.TrimSpace(t.projectDir) == "" && strings.TrimSpace(t.completionMsg) == "" {
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

	var rows []string
	if msg := strings.TrimSpace(t.completionMsg); msg != "" {
		for _, line := range strings.Split(msg, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				rows = append(rows, "")
				continue
			}
			for _, wl := range WrapDisplayWidth(line, inner) {
				rows = append(rows, completionTextStyle.Render(wl))
			}
		}
	}
	if t.projectDir != "" {
		if len(rows) > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, completionTextStyle.Render("Project location:"))
		for _, wl := range WrapDisplayWidth(t.projectDir, inner) {
			rows = append(rows, completionTextStyle.Render("  "+wl))
		}
	} else if len(rows) == 0 {
		return out
	}

	out.Lines = append(out.Lines, "", completionHeaderStyle.Render("✓ Task completed"))

	if len(t.Tasks) > 0 {
		tasks := append([]*TaskEntry(nil), t.Tasks...)
		SortTasksByID(tasks)
		done, _ := taskCounts(tasks)
		chevron := "▸"
		if t.TasksExpanded {
			chevron = "▾"
		}
		out.TasksToggleLine = len(out.Lines)
		toggle := completionToggleStyle.Render(fmt.Sprintf("  %s To-dos %d/%d — click to expand", chevron, done, len(tasks)))
		out.Lines = append(out.Lines, toggle)
		if t.TasksExpanded {
			if box := RenderTodoListBox(tasks, width); box != "" {
				out.Lines = append(out.Lines, box)
			}
		}
	}

	box := completionBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
	out.Lines = append(out.Lines, box, "")
	return out
}

// CompletionPanelLines is a convenience wrapper returning only line strings.
func CompletionPanelLines(t *Turn, width int) []string {
	return CompletionPanelLayout(t, width).Lines
}
