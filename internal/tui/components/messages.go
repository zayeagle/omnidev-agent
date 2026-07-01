package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Message colors
var (
	userMsgColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	agentMsgColor = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	toolMsgColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	errMsgColor   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	helpMsgColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// MessageType classifies a rendered line.
type MessageType int

const (
	MsgUser  MessageType = iota
	MsgAgent
	MsgTool
	MsgError
	MsgHelp
)

// MessageLine is one line in the scrollable message area.
type MessageLine struct {
	Text string
	Type MessageType
}

// MessageList manages the scrollable message buffer.
type MessageList struct {
	lines    []MessageLine
	maxLines int
}

// NewMessageList creates a message list with initial welcome lines.
func NewMessageList(maxLines int) *MessageList {
	ml := &MessageList{
		lines:    make([]MessageLine, 0, maxLines),
		maxLines: maxLines,
	}
	ml.Add(agentMsgColor.Render("  Welcome to omnidev-agent"), MsgAgent)
	ml.Add(agentMsgColor.Render("  Type a natural language command and press Enter."), MsgAgent)
	ml.Add(helpMsgColor.Render("  quit/exit to leave · Ctrl+C interrupt (in session) or exit (idle)"), MsgHelp)
	ml.Add("", MsgHelp)
	return ml
}

// Add appends a styled message line.
func (ml *MessageList) Add(text string, msgType MessageType) {
	ml.lines = append(ml.lines, MessageLine{Text: text, Type: msgType})
	ml.trim()
}

// AddUser adds a user input line.
func (ml *MessageList) AddUser(text string) {
	ml.Add(userMsgColor.Render("> "+text), MsgUser)
}

// AddAgent adds an agent response line.
func (ml *MessageList) AddAgent(text string) {
	for _, line := range strings.Split(text, "\n") {
		ml.Add(agentMsgColor.Render("  "+line), MsgAgent)
	}
}

// AddTool adds a tool invocation line.
func (ml *MessageList) AddTool(text string) {
	ml.Add(toolMsgColor.Render("  ⚙ "+text), MsgTool)
}

// AddToolResult adds a tool result line.
func (ml *MessageList) AddToolResult(success bool, text string) {
	if success {
		ml.Add(toolMsgColor.Render("  ✔ "+text), MsgTool)
	} else {
		ml.Add(errMsgColor.Render("  ✘ "+text), MsgError)
	}
}

// AddError adds an error line.
func (ml *MessageList) AddError(text string) {
	ml.Add(errMsgColor.Render("  ⚡ "+text), MsgError)
}

// AppendLast appends text to the last message line (for streaming).
func (ml *MessageList) AppendLast(text string) {
	if len(ml.lines) == 0 {
		ml.Add(agentMsgColor.Render("  "+text), MsgAgent)
		return
	}
	last := &ml.lines[len(ml.lines)-1]
	last.Text += text
}

// View renders the message area for the given viewport height.
func (ml *MessageList) View(viewportHeight int) string {
	if viewportHeight < 1 {
		viewportHeight = 3
	}

	// Determine visible range
	start := 0
	if len(ml.lines) > viewportHeight {
		start = len(ml.lines) - viewportHeight
	}

	var sb strings.Builder
	for _, l := range ml.lines[start:] {
		sb.WriteString(l.Text + "\n")
	}
	return sb.String()
}

// Lines returns all lines (for full copy/paste).
func (ml *MessageList) Lines() []MessageLine { return ml.lines }

func (ml *MessageList) trim() {
	if len(ml.lines) > ml.maxLines*2 {
		excess := len(ml.lines) - ml.maxLines
		ml.lines = ml.lines[excess:]
	}
}
