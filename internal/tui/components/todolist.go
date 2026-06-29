package components

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	todoHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A1A1AA"))
	todoStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	todoBoxStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#52525B")).
			Padding(0, 1)
	todoItemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	todoIconOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	todoIconFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	todoIconRun     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	todoIconPending = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	todoDependsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	todoErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5"))
)

// SortTasksByID orders tasks by numeric id for top-to-bottom display.
func SortTasksByID(tasks []*TaskEntry) {
	sort.SliceStable(tasks, func(i, j int) bool {
		ai, _ := strconv.Atoi(tasks[i].ID)
		aj, _ := strconv.Atoi(tasks[j].ID)
		if ai != aj {
			return ai < aj
		}
		return tasks[i].ID < tasks[j].ID
	})
}

// RenderTodoList renders a Cursor-style "To-dos N" checklist box.
// liveStatus is shown on the header line after the count (e.g. "Executing").
// When collapseCompleted is true and every task succeeded, returns a one-line summary.
func RenderTodoList(tasks []*TaskEntry, width int, collapseCompleted bool, liveStatus string) []string {
	if len(tasks) == 0 {
		return nil
	}
	tasks = append([]*TaskEntry(nil), tasks...)
	SortTasksByID(tasks)

	if width < 20 {
		width = 60
	}

	done, failed := taskCounts(tasks)
	if collapseCompleted && failed == 0 && done == len(tasks) {
		header := renderTodoHeader(done, len(tasks), liveStatus, true)
		return []string{header, ""}
	}

	header := renderTodoHeader(done, len(tasks), liveStatus, false)
	box := RenderTodoListBox(tasks, width)
	if box == "" {
		return []string{header, ""}
	}
	return []string{header, box, ""}
}

// RenderTodoListBox renders only the bordered task checklist (no header).
func RenderTodoListBox(tasks []*TaskEntry, width int) string {
	if len(tasks) == 0 {
		return ""
	}
	tasks = append([]*TaskEntry(nil), tasks...)
	SortTasksByID(tasks)

	if width < 20 {
		width = 60
	}
	boxWidth := width - 4
	if boxWidth < 30 {
		boxWidth = 30
	}
	innerWidth := boxWidth - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	rows := renderTodoRows(tasks, innerWidth)
	if len(rows) == 0 {
		return ""
	}
	return todoBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
}

func renderTodoRows(tasks []*TaskEntry, innerWidth int) []string {
	var rows []string
	for _, tk := range tasks {
		icon, iconStyle := todoStatusIcon(tk.Status)
		label := tk.Description
		prefix := icon + " "
		prefixWidth := runewidth.StringWidth(prefix)
		prefixRendered := iconStyle.Render(icon) + todoItemStyle.Render(" ")
		wrapped := WrapDisplayWidth(label, innerWidth-prefixWidth)
		for i, wl := range wrapped {
			if i == 0 {
				rows = append(rows, prefixRendered+todoItemStyle.Render(wl))
			} else {
				rows = append(rows, todoItemStyle.Render(strings.Repeat(" ", prefixWidth)+wl))
			}
		}
		if tk.Status == StatusBlocked && len(tk.DependsOn) > 0 {
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
	return rows
}

func renderTodoHeader(done, total int, liveStatus string, allDone bool) string {
	base := fmt.Sprintf("To-dos %d/%d", done, total)
	if allDone {
		base += " · all done"
	}
	var b strings.Builder
	b.WriteString(todoHeaderStyle.Render(base))
	if s := strings.TrimSpace(liveStatus); s != "" {
		b.WriteString(todoStatusStyle.Render(" · Status: " + s))
	}
	return b.String()
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
		return "✓", todoIconOK
	case StatusFailed:
		return "✗", todoIconFail
	case StatusRunning:
		return "◌", todoIconRun
	case StatusBlocked:
		return "⊘", todoIconPending
	default:
		return "○", todoIconPending
	}
}
