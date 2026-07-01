package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func isQuitCommand(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "quit", "exit", ":q", ":quit":
		return true
	default:
		return false
	}
}

func (m *model) isInSession() bool {
	if m.isWorking() {
		return true
	}
	return m.confirming || m.checkpointing || m.planConfirming
}

func (m *model) handleCtrlC() tea.Cmd {
	if m.isInSession() {
		m.interruptSession()
		return nil
	}
	return m.quitSession()
}

func (m *model) quitSession() tea.Cmd {
	m.abortAgentWork()
	m.input.Submit()
	m.persistActiveSession()
	m.quitting = true
	return tea.Quit
}
