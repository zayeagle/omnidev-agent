package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// model holds the TUI state and wired dependencies.
type model struct {
	agent        *agent.Agent
	guard        *agent.ProjectAwarenessGuard
	sessionStore *session.Store
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
	confirmPreview     string
	confirmReply       chan<- permissions.ConfirmResponse
	confirmTimeout     int

	// Checkpoint resume prompt
	checkpointing      bool
	checkpointPhase    string
	checkpointDone     int
	checkpointTotal    int
	checkpointReply    chan<- agent.CheckpointResponse

	// Task plan confirmation (after decomposition)
	planConfirming   bool
	planConfirmReply chan<- agent.TaskPlanConfirmResponse

	// Agent message channel (for continued reading)
	agentCh <-chan tea.Msg

	// Global spinner frame for the Working indicator
	spinnerFrame int
	version      string
	buildTime    string
	agentState   string // latest agent.State string for working label
}

// New creates the top-level TUI model.
func New(a *agent.Agent, cfg *config.Config, guard *agent.ProjectAwarenessGuard, store *session.Store, version, buildTime string) tea.Model {
	m := &model{
		agent:        a,
		guard:        guard,
		sessionStore: store,
		cfg:          cfg,
		input:        components.NewInputLine(),
		turns:        components.NewTurnList(50),
		version:      version,
		buildTime:    buildTime,
	}
	m.restoreFromActiveSession()
	return m
}

func (m *model) restoreFromActiveSession() {
	sess := m.agent.Session()
	if sess == nil || sess.Count() == 0 {
		return
	}
	if ui := sess.UI; ui != nil && len(ui.Turns) > 0 {
		m.turnCount = restoreUI(m.turns, ui)
		if ui.OutputDir != "" {
			m.agent.SetOutputDir(ui.OutputDir)
		}
	} else {
		m.turnCount = hydrateTurnsFromEntries(m.turns, sess.EntriesCopy())
	}
	m.restoreInputHistory()
}

func (m *model) restoreInputHistory() {
	restoreInputHistory(m.input, m.turns)
}

func (m *model) headerInfo() components.HeaderInfo {
	return components.HeaderInfo{
		Version:   m.version,
		BuildTime: m.buildTime,
	}
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
	extra := m.permissionModeLabel()
	if p := m.agent.RunLogPath(); p != "" {
		if extra != "" {
			extra += " · "
		}
		extra += "log: " + p
	}
	return extra
}

func (m *model) dialogOverlayHeight() int {
	w := effectiveWidth(m.width)
	var dialog string
	switch {
	case m.confirming:
		dialog = components.ConfirmDialog(w, m.confirmLevel, m.confirmDescription, m.confirmPreview, m.confirmTimeout)
	case m.checkpointing:
		dialog = components.CheckpointDialog(w, m.checkpointPhase, m.checkpointDone, m.checkpointTotal)
	case m.planConfirming:
		taskCount := 0
		if t := m.currentTurn(); t != nil {
			taskCount = len(t.Tasks)
		}
		dialog = components.PlanConfirmDialog(w, taskCount)
	default:
		return components.ConfirmDialogHeight
	}
	return renderedLineCount(components.ConfirmOverlay(w, dialog))
}

func (m *model) contentViewportHeight() int {
	h := m.height
	if h < 10 {
		h = 24
	}
	reserved := m.headerLineCount() + 2
	if m.confirming || m.checkpointing || m.planConfirming {
		reserved += m.dialogOverlayHeight()
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
