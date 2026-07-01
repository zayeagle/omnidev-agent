package tui

import (
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/commands"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

func (m *model) permissionModeLabel() string {
	if m.agent == nil || m.agent.Permissions().Interactive() {
		return "confirm"
	}
	return "yolo"
}

func (m *model) permissionModeMessage() string {
	if m.agent != nil && !m.agent.Permissions().Interactive() {
		return "Permission mode: yolo (auto-approve all dangerous ops)"
	}
	return "Permission mode: confirm (dangerous ops require approval)"
}

// togglePermissionMode flips confirm ↔ yolo and returns a user-facing summary.
func (m *model) togglePermissionMode() string {
	if m.agent == nil {
		return ""
	}
	c := m.agent.Permissions()
	c.SetInteractive(!c.Interactive())
	return m.permissionModeMessage()
}

// applyPermissionToggle switches mode during an active session (works while agent is running).
func (m *model) applyPermissionToggle() {
	msg := m.togglePermissionMode()
	if msg == "" {
		return
	}
	// Entering yolo while a confirm dialog is open: approve current + allow rest.
	if m.confirming && m.agent != nil && !m.agent.Permissions().Interactive() {
		if m.confirmReply != nil {
			m.confirmReply <- permissions.ConfirmResponse{
				Granted:  true,
				Reason:   "yolo mode",
				AllowAll: true,
			}
		}
		m.confirming = false
		m.confirmReply = nil
	}
	m.notifyPermissionMode(msg)
}

func (m *model) notifyPermissionMode(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	if m.isWorking() {
		if t := m.currentTurn(); t != nil {
			t.AddStatusLine(msg)
			return
		}
	}
	t := m.newTurn("/yolo")
	t.SetCommandOutput(msg)
}

func isSessionSlashCommand(input string) bool {
	return commands.IsBuiltin(input)
}

// applyBuiltinCommand cancels any in-flight agent work and runs a local command.
func (m *model) applyBuiltinCommand(input string) {
	m.abortAgentWork()
	m.input.Submit()
	m.handleBuiltinCommand(input)
}

func (m *model) abortAgentWork() {
	if m.planConfirmReply != nil {
		m.planConfirmReply <- agent.TaskPlanConfirmResponse{Confirmed: false}
		m.planConfirming = false
		m.planConfirmReply = nil
	}
	if m.checkpointReply != nil {
		m.checkpointReply <- agent.CheckpointResponse{Resume: false}
		m.checkpointing = false
		m.checkpointReply = nil
	}
	if m.confirmReply != nil {
		m.confirmReply <- permissions.ConfirmResponse{Granted: false, Reason: "cancelled"}
		m.confirming = false
		m.confirmReply = nil
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.agentCh = nil
	if t := m.currentTurn(); t != nil && t.FinalStatus == components.TurnRunning {
		t.MarkCancelled()
	}
}

func (m *model) handleBuiltinCommand(input string) {
	cmd, args, ok := commands.Parse(input)
	if !ok {
		return
	}
	switch cmd {
	case "help":
		m.showHelp()
	case "clear":
		if !m.isWorking() {
			m.turns = components.NewTurnList(50)
			m.turnCount = 0
			m.input = components.NewInputLine()
		}
	case "archive":
		if !m.isWorking() {
			m.archiveSession()
			t := m.newTurn("/archive")
			t.SetCommandOutput("Session archived. Starting a new conversation.")
		}
	case "sessions":
		m.showSessions()
	case "model":
		t := m.newTurn("/model")
		t.SetCommandOutput("Current model: " + m.cfg.Model + " (" + m.cfg.Provider + ")")
	case "status":
		t := m.newTurn("/status")
		t.SetCommandOutput(NewStatusInfo(m.agent, m.cfg))
	case "checkpoint":
		if strings.HasPrefix(strings.ToLower(args), "rollback ") {
			taskID := strings.TrimSpace(args[len("rollback "):])
			_ = m.startCheckpointRollback(taskID)
		} else {
			t := m.newTurn("/checkpoint")
			t.SetCommandOutput(m.buildCheckpointInfo())
		}
	case "skills":
		m.showSkills()
	case "yolo":
		m.applyPermissionToggle()
	case "copy":
		m.copyScreen()
	case "skill":
		m.loadSkillCommand(args)
	case "session":
		m.showSession(args)
	}
}

// handleSlashCommand is an alias kept for call sites; prefer applyBuiltinCommand.
func (m *model) handleSlashCommand(input string) {
	m.handleBuiltinCommand(input)
}
