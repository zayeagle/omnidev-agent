package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zayeagle/omnidev-agent/internal/version"
)

var (
	headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	headerHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

const agentTagline = "Terminal coding agent — read docs, write code, run tools (with approval)."

const agentCommandsHint = "/help /status /skills /model /yolo /clear /sessions /archive /checkpoint · Ctrl+Y confirm↔yolo · Esc cancel · Y/N/A confirm · ↑↓ history · PgUp/PgDn scroll"

// HeaderInfo holds build metadata shown at the top of the TUI.
type HeaderInfo struct {
	Version    string
	BuildTime  string
	AgentState string // live agent state badge (Thinking, Executing, …)
	Model      string
}

// AgentHeader renders the top banner with width-aware wrapping.
func AgentHeader(info HeaderInfo, width int) string {
	lines := AgentHeaderLines(info, width)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// AgentHeaderLines returns styled header lines for layout height calculation.
func AgentHeaderLines(info HeaderInfo, width int) []string {
	if width < 20 {
		width = 80
	}
	ver := version.Display(info.Version)
	buildTime := strings.TrimSpace(info.BuildTime)
	if buildTime == "" {
		buildTime = "unknown"
	}

	var lines []string
	lines = append(lines, styledWrapLines(headerTitleStyle, "omnidev-agent", width)...)
	lines = append(lines, styledWrapLines(headerHintStyle, agentTagline, width)...)
	lines = append(lines, styledWrapLines(headerHintStyle, "Version "+ver, width)...)
	lines = append(lines, styledWrapLines(headerHintStyle, "Built "+buildTime, width)...)
	lines = append(lines, styledWrapLines(headerHintStyle, "Commands: "+agentCommandsHint, width)...)
	return lines
}

// HeaderLineCount returns how many terminal rows the header occupies at width.
func HeaderLineCount(info HeaderInfo, width int) int {
	n := len(AgentHeaderLines(info, width))
	if n == 0 {
		return 1
	}
	return n
}

func styledWrapLines(style lipgloss.Style, text string, width int) []string {
	wrapped := WrapDisplayWidth(text, width)
	if len(wrapped) == 0 {
		return nil
	}
	out := make([]string, len(wrapped))
	for i, line := range wrapped {
		out[i] = style.Render(line)
	}
	return out
}

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
	return version.Display(info.Version) + " built " + FormatBuildTime(info.BuildTime)
}
