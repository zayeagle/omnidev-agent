package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zayeagle/omnidev-agent/internal/version"
)

var (
	headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	headerHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

const agentTagline = "Terminal coding agent — read docs, write code, run tools (with approval)."

const agentCommandsHint = "/help /status /model /clear /sessions /checkpoint /yolo · Esc cancel · Y/N/A confirm · PgUp/PgDn scroll"

// HeaderInfo holds build metadata shown at the top of the TUI.
type HeaderInfo struct {
	Version   string
	BuildTime string
}

// AgentHeader renders the fixed top banner (name, intro, version, build time, commands).
func AgentHeader(info HeaderInfo) string {
	ver := version.Display(info.Version)
	buildTime := strings.TrimSpace(info.BuildTime)
	if buildTime == "" {
		buildTime = "unknown"
	}

	var b strings.Builder
	b.WriteString(headerTitleStyle.Render("omnidev-agent"))
	b.WriteByte('\n')
	b.WriteString(headerHintStyle.Render(agentTagline))
	b.WriteByte('\n')
	b.WriteString(headerHintStyle.Render("Version "+ver))
	b.WriteByte('\n')
	b.WriteString(headerHintStyle.Render("Built "+buildTime))
	b.WriteByte('\n')
	b.WriteString(headerHintStyle.Render("Commands: " + agentCommandsHint))
	b.WriteByte('\n')
	return b.String()
}

// HeaderLineCount is the number of terminal rows reserved for AgentHeader.
func HeaderLineCount() int { return 5 }

// FormatBuildTime normalizes build timestamps for display.
func FormatBuildTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "unknown" {
		return "unknown"
	}
	return raw
}

// HeaderInfoLabel returns a one-line summary for --version style output.
func HeaderInfoLabel(info HeaderInfo) string {
	return fmt.Sprintf("%s built %s", version.Display(info.Version), FormatBuildTime(info.BuildTime))
}
