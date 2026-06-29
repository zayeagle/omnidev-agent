package components

import (
	"strings"
	"unicode/utf8"
)

// SplitToolDescription splits "tool_name: detail" permission descriptions.
func SplitToolDescription(desc string) (tool, detail string) {
	desc = strings.TrimSpace(desc)
	if i := strings.Index(desc, ":"); i > 0 && i < len(desc)-1 {
		return strings.TrimSpace(desc[:i]), strings.TrimSpace(desc[i+1:])
	}
	return "", desc
}

// TruncateMiddle shortens long single-line text, keeping head and tail.
func TruncateMiddle(s string, max int) string {
	s = strings.TrimSpace(s)
	if max < 12 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	head := max/2 - 1
	tail := max - head - 1
	if head < 4 {
		head = 4
	}
	if tail < 4 {
		tail = 4
	}
	if head+tail+1 >= len(runes) {
		return s
	}
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

// wrapCommandLines wraps command text to width with an optional line cap.
func wrapCommandLines(text string, width, maxLines int) []string {
	if width < 10 {
		width = 10
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > width*maxLines && maxLines > 0 {
			line = TruncateMiddle(line, width*maxLines)
		}
		wrapped := WrapDisplayWidth(line, width)
		for _, wl := range wrapped {
			out = append(out, wl)
			if maxLines > 0 && len(out) >= maxLines {
				if len(wrapped) > len(out) || len(strings.Split(text, "\n")) > 1 {
					out[len(out)-1] = strings.TrimRight(out[len(out)-1], ".") + "…"
				}
				return out
			}
		}
	}
	return out
}
