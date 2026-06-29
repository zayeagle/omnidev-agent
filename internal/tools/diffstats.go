package tools

import (
	"fmt"
	"strings"
)

// LineCount returns the number of lines in s (minimum 1 for non-empty without newline).
func LineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// SnippetLineChange estimates +/- lines for a snippet replace.
func SnippetLineChange(oldSnippet, newSnippet string) (added, removed int) {
	o := LineCount(oldSnippet)
	n := LineCount(newSnippet)
	switch {
	case n > o:
		added = n - o
	case o > n:
		removed = o - n
	case oldSnippet != newSnippet:
		added, removed = 1, 1
	}
	return added, removed
}

// FileLineChange estimates +/- lines for a full file write.
func FileLineChange(oldContent, newContent string) (added, removed int) {
	if oldContent == "" {
		return LineCount(newContent), 0
	}
	o := LineCount(oldContent)
	n := LineCount(newContent)
	if n > o {
		added = n - o
	}
	if o > n {
		removed = o - n
	}
	if o == n && oldContent != newContent {
		added, removed = 1, 1
	}
	return added, removed
}

// FormatChange builds a Cursor-style change summary for tool results.
func FormatChange(verb, path string, added, removed int) string {
	switch {
	case added > 0 && removed > 0:
		return fmt.Sprintf("%s %s (+%d -%d)", verb, path, added, removed)
	case added > 0:
		return fmt.Sprintf("%s %s (+%d)", verb, path, added)
	case removed > 0:
		return fmt.Sprintf("%s %s (-%d)", verb, path, removed)
	default:
		return fmt.Sprintf("%s %s", verb, path)
	}
}
