package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	todoHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	todoBoxStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)
	todoPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	todoRunningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	todoDoneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	todoFailedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	todoBlockedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	todoDependsStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	todoErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5"))
)

// RenderTodoList renders a Cursor-style "To-dos N" checklist box.
// When collapseCompleted is true and every task succeeded, returns a one-line summary.
func RenderTodoList(tasks []*TaskEntry, width int, collapseCompleted bool) []string {
	if len(tasks) == 0 {
		return nil
	}
	if width < 20 {
		width = 60
	}

	done, failed := taskCounts(tasks)
	if collapseCompleted && failed == 0 && done == len(tasks) {
		header := todoHeaderStyle.Render(fmt.Sprintf("To-dos %d/%d — all done", done, len(tasks)))
		return []string{"", header, ""}
	}

	boxWidth := width - 4
	if boxWidth < 30 {
		boxWidth = 30
	}
	innerWidth := boxWidth - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	var rows []string
	for _, tk := range tasks {
		icon, style := todoStatusIcon(tk.Status)
		label := tk.Description
		prefix := icon + " "
		prefixWidth := runewidth.StringWidth(prefix)
		wrapped := WrapDisplayWidth(label, innerWidth-prefixWidth)
		for i, wl := range wrapped {
			if i == 0 {
				rows = append(rows, style.Render(prefix+wl))
			} else {
				rows = append(rows, style.Render(strings.Repeat(" ", prefixWidth)+wl))
			}
		}
		if len(tk.DependsOn) > 0 && tk.Status != StatusSuccess {
			dep := todoDependsStyle.Render(fmt.Sprintf("    waits on: %s", strings.Join(tk.DependsOn, ", ")))
			rows = append(rows, dep)
		}
		if tk.Status == StatusFailed && strings.TrimSpace(tk.Result) != "" {
			errLines := WrapDisplayWidth(tk.Result, innerWidth-4)
			for _, el := range errLines {
				rows = append(rows, todoErrorStyle.Render("    → "+el))
			}
		}
	}

	box := todoBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
	header := todoHeaderStyle.Render(fmt.Sprintf("To-dos %d/%d", done, len(tasks)))

	return []string{"", header, box, ""}
}

func taskCounts(tasks []*TaskEntry) (done, failed int) {
	for _, tk := range tasks {
		switch tk.Status {
		case StatusSuccess:
			done++
		case StatusFailed:
			failed++
		}
	}
	return done, failed
}

func todoStatusIcon(status ItemStatus) (string, lipgloss.Style) {
	switch status {
	case StatusSuccess:
		return "✓", todoDoneStyle
	case StatusFailed:
		return "✗", todoFailedStyle
	case StatusRunning:
		return "◌", todoRunningStyle
	case StatusBlocked:
		return "⊘", todoBlockedStyle
	default:
		return "○", todoPendingStyle
	}
}
