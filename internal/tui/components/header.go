package components

import "github.com/charmbracelet/lipgloss"

var (
	headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	headerHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// AgentHeader renders the welcome title block (empty session).
func AgentHeader(version string) string {
	title := headerTitleStyle.Render("omnidev-agent")
	ver := headerHintStyle.Render("v" + version)
	hint := headerHintStyle.Render("Type a message and press Enter to begin.")
	return title + "\n" + ver + "\n" + hint + "\n"
}

// AgentHeaderCompact is a single-line header during an active session.
func AgentHeaderCompact(version, model string) string {
	line := headerTitleStyle.Render("omnidev-agent") +
		headerHintStyle.Render(" v"+version)
	if model != "" {
		line += headerHintStyle.Render(" · ") + headerHintStyle.Render(model)
	}
	return line + "\n"
}
