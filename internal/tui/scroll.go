package tui

import (
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// scrollExtras bundles transient lines rendered inside the main scroll viewport.
type scrollExtras struct {
	prefix []string
	suffix []string
	panel  components.CompletionPanel
}

func (m *model) scrollExtras() scrollExtras {
	if m.confirming || m.checkpointing || m.planConfirming {
		return scrollExtras{}
	}
	w := effectiveWidth(m.width)
	var ex scrollExtras
	if m.isWorking() {
		ex.suffix = append(ex.suffix,
			components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), w),
		)
	}
	ex.panel = components.CompletionPanelLayout(m.currentTurn(), w)
	if len(ex.panel.Lines) > 0 {
		ex.suffix = append(ex.suffix, ex.panel.Lines...)
	}
	return ex
}

func (m *model) scrollViewportHeight() int {
	return m.contentViewportHeight()
}

func (m *model) scrollPrefix() []string { return m.scrollExtras().prefix }
func (m *model) scrollSuffix() []string {
	return m.scrollExtras().suffix
}

func (m *model) scrollUp(lines int) {
	ex := m.scrollExtras()
	vh := m.scrollViewportHeight()
	m.turns.ScrollUp(lines, vh, ex.prefix, ex.suffix)
}

func (m *model) scrollDown(lines int) {
	ex := m.scrollExtras()
	vh := m.scrollViewportHeight()
	m.turns.ScrollDown(lines, vh, ex.prefix, ex.suffix)
}

func (m *model) scrollHint() string {
	ex := m.scrollExtras()
	return m.turns.ScrollHint(m.scrollViewportHeight(), ex.prefix, ex.suffix)
}
