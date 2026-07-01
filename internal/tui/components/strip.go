package components

import "regexp"

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes terminal color/style sequences from rendered TUI lines.
func StripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}
