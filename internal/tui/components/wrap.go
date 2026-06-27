package components

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// WrapDisplayWidth wraps text to fit terminal display width (CJK-aware).
func WrapDisplayWidth(text string, width int) []string {
	if width <= 0 {
		if text == "" {
			return nil
		}
		return []string{text}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		lines = append(lines, wrapDisplayParagraph(paragraph, width)...)
	}
	if len(lines) == 0 && text != "" {
		return []string{text}
	}
	return lines
}

func wrapDisplayParagraph(s string, width int) []string {
	if s == "" {
		return nil
	}
	if runewidth.StringWidth(s) <= width {
		return []string{s}
	}
	var lines []string
	remaining := s
	for runewidth.StringWidth(remaining) > width {
		cut := cutDisplayWidth(remaining, width)
		if cut <= 0 {
			cut = 1
		}
		chunk := remaining[:cut]
		lines = append(lines, strings.TrimRight(chunk, " "))
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return lines
}

func cutDisplayWidth(s string, width int) int {
	w := 0
	lastSpace := -1
	for i, r := range s {
		if r == ' ' {
			lastSpace = i
		}
		rw := runewidth.RuneWidth(r)
		if w+rw > width {
			if lastSpace > 0 {
				return lastSpace
			}
			if i == 0 {
				return len(string(r))
			}
			return i
		}
		w += rw
	}
	return len(s)
}
