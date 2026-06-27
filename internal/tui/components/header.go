package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	headerHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

const agentTagline = "Terminal coding agent — read docs, write code, run tools (with approval)."

const agentCommandsHint = "Commands: /help /status /model /clear /sessions · Esc cancel · Y/N/A confirm · PgUp/PgDn scroll"

// HeaderInfo holds build metadata shown at the top of the TUI.
type HeaderInfo struct {
	Version   string
	BuildTime string
}

// AgentHeader renders the welcome title block (empty session).
func AgentHeader(info HeaderInfo) string {
	return renderHeader(info, false)
}

// AgentHeaderCompact is a shorter header during an active session (no model name).
func AgentHeaderCompact(info HeaderInfo) string {
	return renderHeader(info, true)
}

// HeaderLineCount returns how many terminal rows the header occupies.
func HeaderLineCount(compact bool) int {
	if compact {
		return 3
	}
	return 5
}

func renderHeader(info HeaderInfo, compact bool) string {
	version := strings.TrimSpace(info.Version)
	if version == "" {
		version = "dev"
	}
	buildTime := strings.TrimSpace(info.BuildTime)
	if buildTime == "" {
		buildTime = "unknown"
	}

	title := headerTitleStyle.Render("omnidev-agent")
	meta := headerHintStyle.Render(fmt.Sprintf("Version v%s    Built %s", version, buildTime))
	commands := headerHintStyle.Render(agentCommandsHint)

	if compact {
		return title + "\n" + meta + "\n" + commands + "\n"
	}

	tagline := headerHintStyle.Render(agentTagline)
	return title + "\n" + tagline + "\n" + meta + "\n" + commands + "\n"
}
