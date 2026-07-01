package tui

import (
	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// interruptSession stops in-flight work and preserves checkpoint for follow-up (Ctrl+C).
func (m *model) interruptSession() {
	if m.planConfirming {
		if m.planConfirmReply != nil {
			m.planConfirmReply <- agent.TaskPlanConfirmResponse{Confirmed: false}
		}
		m.planConfirming = false
		m.planConfirmReply = nil
	}
	if m.checkpointing {
		m.checkpointing = false
		m.checkpointReply = nil
	}
	if m.confirming {
		if m.confirmReply != nil {
			m.confirmReply <- permissions.ConfirmResponse{Granted: false, Reason: "interrupted"}
		}
		m.confirming = false
		m.confirmReply = nil
	}
	if !m.isWorking() {
		return
	}
	m.agent.SaveInterruptCheckpoint(0)
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.agentCh = nil
	if t := m.currentTurn(); t != nil && t.FinalStatus == components.TurnRunning {
		t.MarkInterrupted()
	}
	m.persistActiveSession()
}
