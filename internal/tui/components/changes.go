package components

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// FileChange tracks line deltas for one path in a turn.
type FileChange struct {
	Path    string
	Added   int
	Removed int
}

var changeSummaryRE = regexp.MustCompile(`(?i)(?:wrote|edited|deleted)\s+(\S+)\s*(?:\(\+(\d+)(?:\s+-(\d+))?\)|\(-(\d+)\))?`)

// ParseChangeFromToolResult extracts path and +/- lines from tool output.
func ParseChangeFromToolResult(toolName, data string) (FileChange, bool) {
	data = strings.TrimSpace(data)
	if data == "" {
		return FileChange{}, false
	}
	if m := changeSummaryRE.FindStringSubmatch(data); len(m) > 0 {
		fc := FileChange{Path: normalizeChangePath(m[1])}
		if m[2] != "" {
			fmt.Sscanf(m[2], "%d", &fc.Added)
		}
		if len(m) > 3 && m[3] != "" {
			fmt.Sscanf(m[3], "%d", &fc.Removed)
		}
		if len(m) > 4 && m[4] != "" {
			fmt.Sscanf(m[4], "%d", &fc.Removed)
		}
		return fc, fc.Path != ""
	}
	switch toolName {
	case "write_file", "edit_file", "delete_file":
		parts := strings.Fields(data)
		if len(parts) >= 2 {
			return FileChange{Path: normalizeChangePath(parts[len(parts)-1])}, true
		}
	}
	return FileChange{}, false
}

func normalizeChangePath(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	return p
}

func (t *Turn) RecordFileChange(fc FileChange) {
	fc.Path = normalizeChangePath(fc.Path)
	if fc.Path == "" {
		return
	}
	for i := range t.fileChanges {
		if t.fileChanges[i].Path == fc.Path {
			t.fileChanges[i].Added += fc.Added
			t.fileChanges[i].Removed += fc.Removed
			return
		}
	}
	t.fileChanges = append(t.fileChanges, fc)
}

func (t *Turn) FileChanges() []FileChange {
	return append([]FileChange(nil), t.fileChanges...)
}

var (
	changeHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true)
	changePathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	changeAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	changeDelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
)

// ChangePanelLines renders a summary of files modified in this turn.
func ChangePanelLines(t *Turn, width int) []string {
	if t == nil || len(t.fileChanges) == 0 {
		return nil
	}
	if width < 30 {
		width = 80
	}
	cw := width - 2
	changes := append([]FileChange(nil), t.fileChanges...)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })

	var lines []string
	lines = append(lines, changeHeaderStyle.Render("Files changed:"))
	for _, fc := range changes {
		for _, row := range renderChangeRow(fc, cw) {
			lines = append(lines, row)
		}
	}
	lines = append(lines, "")
	return lines
}

func renderChangeRow(fc FileChange, width int) []string {
	path := normalizeChangePath(fc.Path)
	statPlain := formatChangeStatPlain(fc.Added, fc.Removed)
	statWidth := runewidth.StringWidth(statPlain)
	indent := "  "
	gap := 2
	pathMax := width - runewidth.StringWidth(indent) - statWidth - gap
	if pathMax < 16 {
		pathMax = 16
	}
	if runewidth.StringWidth(path) > pathMax {
		path = TruncateMiddle(path, pathMax)
	}
	pad := pathMax - runewidth.StringWidth(path)
	if pad < gap {
		pad = gap
	}
	pathPart := changePathStyle.Render(path)
	statPart := renderChangeStatStyled(fc.Added, fc.Removed)
	line := indent + pathPart + strings.Repeat(" ", pad) + statPart
	return []string{line}
}

func formatChangeStatPlain(added, removed int) string {
	switch {
	case added > 0 && removed > 0:
		return fmt.Sprintf("+%d  -%d", added, removed)
	case added > 0:
		return fmt.Sprintf("+%d", added)
	case removed > 0:
		return fmt.Sprintf("-%d", removed)
	default:
		return "·"
	}
}

func renderChangeStatStyled(added, removed int) string {
	switch {
	case added > 0 && removed > 0:
		return changeAddStyle.Render(fmt.Sprintf("+%d", added)) + "  " + changeDelStyle.Render(fmt.Sprintf("-%d", removed))
	case added > 0:
		return changeAddStyle.Render(fmt.Sprintf("+%d", added))
	case removed > 0:
		return changeDelStyle.Render(fmt.Sprintf("-%d", removed))
	default:
		return changePathStyle.Render("·")
	}
}
