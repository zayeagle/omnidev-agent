package tui

import (
	"strings"

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
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return false
	}
	switch input {
	case "/help", "/status", "/model", "/yolo", "/skills", "/checkpoint", "/clear", "/sessions", "/archive":
		return true
	}
	return strings.HasPrefix(input, "/skill ") ||
		strings.HasPrefix(input, "/session ") ||
		strings.HasPrefix(input, "/checkpoint ")
}

func (m *model) handleSlashCommand(input string) {
	switch input {
	case "/help":
		m.showHelp()
	case "/clear":
		if !m.isWorking() {
			m.turns = components.NewTurnList(50)
			m.turnCount = 0
			m.input = components.NewInputLine()
		}
	case "/archive":
		if !m.isWorking() {
			m.archiveSession()
			t := m.newTurn("/archive")
			t.SetCommandOutput("Session archived. Starting a new conversation.")
		}
	case "/sessions":
		m.showSessions()
	case "/model":
		t := m.newTurn("/model")
		t.SetCommandOutput("Current model: " + m.cfg.Model + " (" + m.cfg.Provider + ")")
	case "/status":
		t := m.newTurn("/status")
		t.SetCommandOutput(NewStatusInfo(m.agent, m.cfg))
	case "/checkpoint":
		t := m.newTurn("/checkpoint")
		t.SetCommandOutput(m.buildCheckpointInfo())
	case "/skills":
		m.showSkills()
	case "/yolo":
		m.applyPermissionToggle()
	default:
		if strings.HasPrefix(input, "/skill ") {
			m.loadSkillCommand(strings.TrimSpace(strings.TrimPrefix(input, "/skill ")))
		} else if strings.HasPrefix(input, "/session ") {
			m.showSession(strings.TrimSpace(strings.TrimPrefix(input, "/session ")))
		} else if strings.HasPrefix(input, "/checkpoint ") {
			sub := strings.TrimSpace(strings.TrimPrefix(input, "/checkpoint"))
			if strings.HasPrefix(sub, "rollback ") {
				taskID := strings.TrimSpace(strings.TrimPrefix(sub, "rollback "))
				_ = m.startCheckpointRollback(taskID)
			}
		}
	}
}
