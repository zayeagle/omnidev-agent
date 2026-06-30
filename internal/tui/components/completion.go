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
// TasksToggleLine / AcceptanceToggleLine are 0-based line indices for click-to-expand, or -1.
type CompletionPanel struct {
	Lines                 []string
	TasksToggleLine       int
	AcceptanceToggleLine  int
}

// CompletionPanelLayout builds completion UI below the transcript.
func CompletionPanelLayout(t *Turn, width int) CompletionPanel {
	out := CompletionPanel{TasksToggleLine: -1, AcceptanceToggleLine: -1}
	if t == nil || t.IsChatMode() {
		return out
	}
	if strings.TrimSpace(t.projectDir) == "" && strings.TrimSpace(t.completionMsg) == "" && strings.TrimSpace(t.acceptanceDetail) == "" {
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
	hasAcceptanceDetail := strings.TrimSpace(t.acceptanceDetail) != ""

	if hasAcceptanceDetail {
		if t.AcceptanceExpanded {
			for _, line := range strings.Split(t.acceptanceDetail, "\n") {
				line = strings.TrimRight(line, " \t")
				if line == "" {
					rows = append(rows, "")
					continue
				}
				for _, wl := range WrapDisplayWidth(line, inner) {
					rows = append(rows, completionTextStyle.Render(wl))
				}
			}
		}
	} else if msg := strings.TrimSpace(t.completionMsg); msg != "" {
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
	} else if len(rows) == 0 && !hasAcceptanceDetail {
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

	if hasAcceptanceDetail {
		chevron := "▸"
		if t.AcceptanceExpanded {
			chevron = "▾"
		}
		label := fmt.Sprintf("  %s 验收通过 %d/%d · 查看详细", chevron, t.acceptancePassedN, t.acceptanceTotalN)
		if !t.acceptancePassed {
			label = fmt.Sprintf("  %s 验收未通过 %d/%d · 查看详细", chevron, t.acceptancePassedN, t.acceptanceTotalN)
		}
		out.AcceptanceToggleLine = len(out.Lines)
		out.Lines = append(out.Lines, completionToggleStyle.Render(label))
	}

	if len(rows) > 0 {
		box := completionBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
		out.Lines = append(out.Lines, box)
	}
	out.Lines = append(out.Lines, "")
	return out
}

// CompletionPanelLines is a convenience wrapper returning only line strings.
func CompletionPanelLines(t *Turn, width int) []string {
	return CompletionPanelLayout(t, width).Lines
}
