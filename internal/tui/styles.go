package tui

import "github.com/charmbracelet/lipgloss"

// Color palette — Omnidev theme
var (
	// Core
	violet     = lipgloss.Color("#7C3AED")
	violetDark = lipgloss.Color("#1E1B4B")
	violetMid  = lipgloss.Color("#A78BFA")
	violetLight= lipgloss.Color("#DDD6FE")

	// Semantic
	green  = lipgloss.Color("#34D399")
	blue   = lipgloss.Color("#60A5FA")
	yellow = lipgloss.Color("#FBBF24")
	red    = lipgloss.Color("#F87171")
	gray   = lipgloss.Color("#6B7280")
	white  = lipgloss.Color("#F9FAFB")

	// Border
	borderGray = lipgloss.Color("#374151")
)

// ── Base styles ──

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(violet).Background(violetDark).Padding(0, 1)
	StatusStyle = lipgloss.NewStyle().Foreground(violetMid).Italic(true)
	SepStyle    = lipgloss.NewStyle().Foreground(borderGray)

	UserStyle   = lipgloss.NewStyle().Foreground(green)
	AgentStyle  = lipgloss.NewStyle().Foreground(blue)
	ToolStyle   = lipgloss.NewStyle().Foreground(yellow)
	ErrStyle    = lipgloss.NewStyle().Foreground(red)
	PromptStyle = lipgloss.NewStyle().Foreground(violet).Bold(true)
	HelpStyle   = lipgloss.NewStyle().Foreground(gray)

	// Confirmation dialog
	ConfirmBorderStyle = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(red)
	ConfirmTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(red)
	ConfirmDescStyle   = lipgloss.NewStyle().Foreground(white)
	ConfirmKeyStyle    = lipgloss.NewStyle().Bold(true).Foreground(green)

	// Status label colors
	StatusIdle            = lipgloss.NewStyle().Foreground(gray)
	StatusThinking        = lipgloss.NewStyle().Foreground(violetMid)
	StatusExecuting       = lipgloss.NewStyle().Foreground(yellow)
	StatusWaitingApproval = lipgloss.NewStyle().Foreground(red)
	StatusDone            = lipgloss.NewStyle().Foreground(green)
	StatusError           = lipgloss.NewStyle().Foreground(red)

	// Message padding
	MsgPadding = lipgloss.NewStyle().PaddingLeft(2)
)

// StatusColor returns the lipgloss style for the given state string.
func StatusColor(state string) lipgloss.Style {
	switch state {
	case "Idle":
		return StatusIdle
	case "Thinking":
		return StatusThinking
	case "Executing":
		return StatusExecuting
	case "WaitingApproval":
		return StatusWaitingApproval
	case "Done":
		return StatusDone
	case "Error":
		return StatusError
	default:
		return HelpStyle
	}
}
