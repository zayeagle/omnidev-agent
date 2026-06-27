package agent

import "github.com/zayeagle/omnidev-agent/internal/permissions"

// ── TUI → Agent ─────────────────────────────────────────────────────────────

// UserInputMsg carries a natural language instruction from the user.
type UserInputMsg struct {
	Instruction string
}

// ConfirmResponseMsg carries the user's decision on a permission prompt.
type ConfirmResponseMsg struct {
	Granted bool
	Reason  string
}

// ── Agent → TUI ─────────────────────────────────────────────────────────────

// AgentStateMsg notifies the TUI of a state transition.
type AgentStateMsg struct {
	State State
}

// StreamChunkMsg carries one chunk of streaming LLM output.
type StreamChunkMsg struct {
	Content string
	Done    bool // true when this is the final chunk
}

// ToolCallMsg notifies the TUI that a tool is being invoked.
type ToolCallMsg struct {
	Name   string
	Args   map[string]interface{}
	Status string // "executing" | "awaiting_approval"
}

// ToolResultMsg carries the result of a completed tool execution.
type ToolResultMsg struct {
	Success bool
	Data    string
	Error   string
}

// ConfirmRequestMsg asks the TUI to display a permission dialog.
// The agent blocks on Reply until the user responds.
type ConfirmRequestMsg struct {
	Level       permissions.Level
	Description string
	Reply       chan<- permissions.ConfirmResponse
}

// ErrorMsg notifies the TUI of a recoverable error.
type ErrorMsg struct {
	Error string
	Retry int // retry count (1-based), 0 = no retry
}

// DoneMsg signals that the agent loop has completed.
type DoneMsg struct{}

// TaskPlanItem is one row in the task checklist shown in the TUI.
type TaskPlanItem struct {
	ID          string
	Description string
}

// TaskPlanMsg sends the full decomposed task list to the TUI at once.
type TaskPlanMsg struct {
	Tasks []TaskPlanItem
}

// AllCompleteMsg signals that every sub-task finished successfully.
type AllCompleteMsg struct {
	Summary    string
	ProjectDir string
}
