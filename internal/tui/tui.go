package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// model holds the TUI state and wired dependencies.
type model struct {
	agent        *agent.Agent
	guard        *agent.ProjectAwarenessGuard
	cfg    *config.Config
	ctx    context.Context
	cancel context.CancelFunc

	width  int
	height int

	input    *components.InputLine
	turns    *components.TurnList
	quitting bool

	turnCount int

	// Permission confirmation state
	confirming         bool
	confirmLevel       string
	confirmDescription string
	confirmReply       chan<- permissions.ConfirmResponse
	confirmTimeout     int

	// Agent message channel (for continued reading)
	agentCh <-chan tea.Msg

	// Global spinner frame for the Working indicator
	spinnerFrame int
	version      string
	buildTime    string
	agentState   string // latest agent.State string for working label
}

// New creates the top-level TUI model.
func New(a *agent.Agent, cfg *config.Config, guard *agent.ProjectAwarenessGuard, version, buildTime string) tea.Model {
	return &model{
		agent:        a,
		guard:        guard,
		cfg:          cfg,
		input:        components.NewInputLine(),
		turns:        components.NewTurnList(50),
		version:      version,
		buildTime:    buildTime,
	}
}

func (m *model) headerInfo() components.HeaderInfo {
	return components.HeaderInfo{Version: m.version, BuildTime: m.buildTime}
}

// Init is called once when the program starts.
func (m *model) Init() tea.Cmd {
	return nil
}

// currentTurn returns the currently active turn, or nil.
func (m *model) currentTurn() *components.Turn {
	return m.turns.LastTurn()
}

// newTurn creates a turn and assigns it an ID.
func (m *model) newTurn(input string) *components.Turn {
	m.turnCount++
	t := m.turns.AddTurn(m.turnCount, input)
	t.SetActive(true)
	return t
}

// isWorking reports whether the agent loop is still running for the current turn.
func (m *model) isWorking() bool {
	if m.agentCh != nil {
		return true
	}
	t := m.currentTurn()
	if t == nil {
		return false
	}
	return t.FinalStatus == components.TurnRunning && t.IsActive()
}

// workingLabel returns the Cursor-style working indicator text.
func (m *model) workingLabel() string {
	if t := m.currentTurn(); t != nil {
		return t.WorkingLabel(m.agentState)
	}
	if m.agentState != "" {
		return m.agentState
	}
	return "Working"
}

func (m *model) footerExtra() string {
	if m.agent != nil && !m.agent.Permissions().Interactive() {
		return "yolo"
	}
	return ""
}

// pinTasksTurn returns the active turn whose task list is pinned above the scroll area.
func (m *model) pinTasksTurn() *components.Turn {
	t := m.currentTurn()
	if t == nil || len(t.Tasks) == 0 {
		return nil
	}
	if m.isWorking() || t.HasCompletion() || t.FinalStatus == components.TurnDone {
		return t
	}
	return nil
}
func (m *model) contentViewportHeight() int {
	h := m.height
	if h < 10 {
		h = 24
	}
	reserved := m.headerLineCount() + 2
	if m.confirming {
		reserved += components.ConfirmDialogHeight
		if m.isWorking() {
			reserved += renderedLineCount(components.WorkingIndicator(m.spinnerFrame, m.workingLabel(), effectiveWidth(m.width)))
		}
	} else {
		reserved += m.chromeLineCount(m.isWorking())
	}
	msgHeight := h - reserved
	if msgHeight < 3 {
		msgHeight = 3
	}
	return msgHeight
}

// transcriptViewportHeight is the scrollable area below a pinned To-dos panel.
func (m *model) transcriptViewportHeight() int {
	h := m.contentViewportHeight()
	if pin := m.pinTasksTurn(); pin != nil {
		w := effectiveWidth(m.width)
		sticky := len(components.TaskPanelLines(pin, w)) + 1
		h -= sticky
		if h < 3 {
			h = 3
		}
	}
	return h
}

// ── Internal message types ──

// agentMsgBatch bundles an agent message with its source channel for continued reading.
type agentMsgBatch struct {
	msg tea.Msg
	ch  <-chan tea.Msg
}

// agentLoopStartedMsg signals that the agent goroutine has been spawned and carries its message channel.
type agentLoopStartedMsg struct {
	ch <-chan tea.Msg
}

// confirmTickMsg triggers the permission approval countdown each second.
type confirmTickMsg struct{}

// spinnerTickMsg triggers spinner animation frame advance.
type spinnerTickMsg struct{}

// agentLoopEndedMsg signals the agent message channel has closed.
type agentLoopEndedMsg struct{}
