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

	if m.turns.Count() > 0 {
		pinTasks := m.pinTasksTurn()
		if pinTasks != nil {
			panel := components.TaskPanelLines(pinTasks, w)
			if len(panel) > 0 {
				b.WriteString(strings.Join(panel, "\n"))
				b.WriteString("\n")
			}
		}
		scrollH := m.transcriptViewportHeight()
		b.WriteString(m.turns.View(scrollH, pinTasks))
	}

	b.WriteString("\n")

	working := m.isWorking()
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
	} else {
		if working {
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), w))
			b.WriteString("\n")
		}
		if cp := components.CompletionPanelLines(m.currentTurn(), w); len(cp) > 0 {
			b.WriteString(strings.Join(cp, "\n"))
		}
		b.WriteString(m.input.View(working, m.turns.Count() > 0, w))
		b.WriteString("\n")
		b.WriteString(components.FooterBar(
			w,
			modelName,
			contextUsagePct(m.agent),
			m.turns.ScrollHint(m.transcriptViewportHeight()),
			m.footerExtra(),
		))
	}

	_ = h // height drives transcriptViewportHeight via layout helpers
	return b.String()
}
