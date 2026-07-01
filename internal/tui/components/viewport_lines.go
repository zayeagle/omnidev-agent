package components

import "strings"

// FlattenViewportLines expands embedded newlines so one scroll index equals one terminal row.
func FlattenViewportLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	var out []string
	for _, block := range lines {
		if block == "" {
			out = append(out, "")
			continue
		}
		parts := strings.Split(block, "\n")
		out = append(out, parts...)
	}
	return out
}

// VisualLineCount returns terminal rows for a slice that may contain embedded newlines.
func VisualLineCount(lines []string) int {
	return len(FlattenViewportLines(lines))
}
