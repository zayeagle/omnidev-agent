package tui

import (
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// View renders the full TUI layout (Cursor Agent style).
func (m *model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	w := effectiveWidth(m.width)
	h := m.height
	if h < 10 {
		h = 24
	}

	m.turns.SetWidth(w)

	modelName := m.cfg.Model
	if modelName == "" {
		modelName = "Auto"
	}

	var b strings.Builder

	b.WriteString(components.AgentHeader(m.headerInfo(), w))

	working := m.isWorking()
	inDialog := m.confirming || m.checkpointing || m.planConfirming

	if m.turns.Count() > 0 && !inDialog {
		ex := m.scrollExtras()
		scrollH := m.scrollViewportHeight()
		visible, _ := m.turns.ViewScroll(scrollH, ex.prefix, ex.suffix)
		if visible != "" {
			b.WriteString(visible)
		}
	}

	b.WriteString("\n")

	if m.confirming {
		if working {
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), w))
			b.WriteString("\n")
		}
		dialog := components.ConfirmDialog(w, m.confirmLevel, m.confirmDescription, m.confirmPreview, m.confirmTimeout)
		b.WriteString(components.ConfirmOverlay(w, dialog))
	} else if m.checkpointing {
		if working {
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), w))
			b.WriteString("\n")
		}
		dialog := components.CheckpointDialog(w, m.checkpointPhase, m.checkpointDone, m.checkpointTotal)
		b.WriteString(components.ConfirmOverlay(w, dialog))
	} else if m.planConfirming {
		if working {
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), w))
			b.WriteString("\n")
		}
		taskCount := 0
		if t := m.currentTurn(); t != nil {
			taskCount = len(t.Tasks)
		}
		dialog := components.PlanConfirmDialog(w, taskCount)
		b.WriteString(components.ConfirmOverlay(w, dialog))
	} else {
		b.WriteString(m.input.View(working, m.turns.Count() > 0, w))
		b.WriteString("\n")
		b.WriteString(components.FooterBar(
			w,
			modelName,
			contextUsagePct(m.agent),
			m.scrollHint(),
			m.footerExtra(),
		))
		if hint := components.FooterExitHint(w, m.isInSession()); hint != "" {
			b.WriteString("\n")
			b.WriteString(hint)
		}
	}

	_ = h // height drives contentViewportHeight via layout helpers
	return b.String()
}
