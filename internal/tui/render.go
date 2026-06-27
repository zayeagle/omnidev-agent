package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// View renders the full TUI layout (Cursor Agent style).
func (m *model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	w := m.width
	if w < 20 {
		w = 80
	}
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

	info := m.headerInfo()
	compactHeader := m.turns.Count() > 0
	headerRows := components.HeaderLineCount(compactHeader)
	if compactHeader {
		b.WriteString(components.AgentHeaderCompact(info))
	} else {
		b.WriteString(components.AgentHeader(info))
	}

	// Reserve: header + working(0-1) + input(1) + footer(1) + gaps(2)
	reserved := headerRows + 4
	if m.isWorking() {
		reserved++
	}
	if m.confirming {
		reserved = headerRows + 1 + components.ConfirmDialogHeight
		if m.isWorking() {
			reserved++
		}
	}
	msgHeight := h - reserved
	if msgHeight < 3 {
		msgHeight = 3
	}

	if m.turns.Count() == 0 {
		b.WriteString(welcomeBanner(w))
	} else {
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
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel()))
			b.WriteString("\n")
		}
		dialog := components.ConfirmDialog(w, m.confirmLevel, m.confirmDescription, m.confirmTimeout)
		b.WriteString(components.ConfirmOverlay(w, dialog))
	} else {
		if working {
			b.WriteString(components.WorkingIndicator(m.spinnerFrame, m.workingLabel()))
			b.WriteString("\n")
		}
		if cp := components.CompletionPanelLines(m.currentTurn(), w); len(cp) > 0 {
			b.WriteString(strings.Join(cp, "\n"))
		}
		b.WriteString(m.input.View(working))
		b.WriteString("\n")
		b.WriteString(components.FooterBar(
			modelName,
			contextUsagePct(m.agent),
			m.turns.ScrollHint(m.transcriptViewportHeight()),
			m.footerExtra(),
		))
	}

	return b.String()
}

var (
	welcomeTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	welcomeTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

func welcomeBanner(width int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(welcomeTitleStyle.Render("  Examples"))
	sb.WriteString("\n")
	sb.WriteString(welcomeTextStyle.Render("  > implement a hello-world HTTP server"))
	sb.WriteString("\n")
	sb.WriteString(welcomeTextStyle.Render("  > explain how the agent loop works"))
	return sb.String()
}
