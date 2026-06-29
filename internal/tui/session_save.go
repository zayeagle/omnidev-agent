package tui

import (
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

func (m *model) persistActiveSession() {
	if m.sessionStore == nil {
		return
	}
	sess := m.agent.Session()
	if sess == nil {
		return
	}
	sess.UI = snapshotUI(m.turns, m.turnCount, m.agent.OutputDir())
	_ = m.sessionStore.SaveActive(sess)
}

func (m *model) archiveSession() {
	if m.isWorking() || m.sessionStore == nil {
		return
	}
	sess := m.agent.Session()
	if sess == nil {
		return
	}
	sess.UI = snapshotUI(m.turns, m.turnCount, m.agent.OutputDir())
	if sess.Count() > 0 {
		_ = m.sessionStore.Archive(sess)
	} else {
		_ = m.sessionStore.ClearActive()
	}

	newSess := session.New()
	m.agent.SetSession(newSess)
	if m.guard != nil {
		m.guard.SetSession(newSess)
	}
	m.turns = components.NewTurnList(50)
	m.turnCount = 0
}
