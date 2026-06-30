package agent

import tea "github.com/charmbracelet/bubbletea"

func emitActivity(msgCh chan<- tea.Msg, detail string) {
	if msgCh == nil || detail == "" {
		return
	}
	msgCh <- ActivityMsg{Detail: detail}
}
