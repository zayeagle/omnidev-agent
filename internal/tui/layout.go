package tui

import (
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

func effectiveWidth(w int) int {
	if w < 20 {
		return 80
	}
	return w
}

func renderedLineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// visualLineCount counts terminal rows, ignoring a trailing newline.
func visualLineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m *model) headerLineCount() int {
	return components.HeaderLineCount(m.headerInfo(), effectiveWidth(m.width))
}

func (m *model) chromeLineCount(working bool) int {
	w := effectiveWidth(m.width)
	n := renderedLineCount(m.input.View(working, m.turns.Count() > 0, w))
	modelName := m.cfg.Model
	if modelName == "" {
		modelName = "Auto"
	}
	n += renderedLineCount(components.FooterBar(
		w,
		modelName,
		contextUsagePct(m.agent),
		"", // avoid recursive layout via scroll hint
		m.footerExtra(),
	))
	n += renderedLineCount(components.FooterExitHint(w, m.isInSession()))
	return n
}
