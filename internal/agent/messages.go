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

// ActivityMsg reports what the agent is doing right now (shown in the working indicator).
type ActivityMsg struct {
	Detail string
}

// StreamChunkMsg carries one chunk of streaming LLM output.
type StreamChunkMsg struct {
	Content string
	Done    bool // true when this is the final chunk
}

// ToolCallMsg notifies the TUI that a tool is being invoked.
type ToolCallMsg struct {
	Name      string
	Args      map[string]interface{}
	Status    string // "executing" | "awaiting_approval"
	SubtaskID string // non-empty when invoked by a parallel sub-agent
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
	Preview     string // optional diff/snippet for write/edit/delete
	Reply       chan<- permissions.ConfirmResponse
}

// CheckpointPromptMsg asks whether to resume an in-progress checkpoint.
type CheckpointPromptMsg struct {
	Phase                string
	Completed            int
	Total                int
	AcceptanceIncomplete bool
	Reply                chan<- CheckpointResponse
}

// CheckpointResponse is the user's decision on checkpoint resume.
type CheckpointResponse struct {
	Resume bool // true = resume, false = start fresh
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
	DependsOn   []string
}

// TaskPlanMsg sends the full decomposed task list to the TUI at once.
type TaskPlanMsg struct {
	Tasks []TaskPlanItem
}

// TaskPlanConfirmMsg asks the user to approve the task plan before execution.
type TaskPlanConfirmMsg struct {
	TaskCount int
	Reply     chan<- TaskPlanConfirmResponse
}

// TaskPlanConfirmResponse is the user's decision on the task plan.
type TaskPlanConfirmResponse struct {
	Confirmed bool // true = proceed, false = cancel
}

// AllCompleteMsg signals that every sub-task finished successfully.
type AllCompleteMsg struct {
	Summary            string
	ProjectDir         string
	AcceptanceDetail   string // full formatAcceptanceReport; shown when user expands
	AcceptancePassed   bool
	AcceptancePassedN  int
	AcceptanceTotalN   int
}

// VerificationProgressMsg reports acceptance-criteria progress during verify phase.
type VerificationProgressMsg struct {
	Passed   int
	Total    int
	Criteria []CriterionStatus
	Detail   string // full per-criterion report (legacy / headless)
	AllMet   bool
	InitChecklist bool // true: show all criteria as pending (○)
	CheckedIndex  int  // >=0: one criterion just finished checking
	AppendText    string // optional extra criterion row (mechanical verify)
	Finalize      bool   // true: verification round complete
}

// PartialCompleteMsg signals incomplete or failed completion (no success banner).
type PartialCompleteMsg struct {
	Summary            string
	ProjectDir         string
	Criteria           []CriterionStatus
	Reason             string
	Resumable          bool // checkpoint preserved; user can resume
	AcceptanceDetail   string
	AcceptancePassed   bool
	AcceptancePassedN  int
	AcceptanceTotalN   int
}
