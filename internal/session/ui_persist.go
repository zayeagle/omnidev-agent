package session

// PersistedUI stores TUI turn state for resume across restarts.
type PersistedUI struct {
	TurnCount int             `json:"turn_count"`
	OutputDir string          `json:"output_dir,omitempty"`
	Turns     []PersistedTurn `json:"turns,omitempty"`
}

// PersistedTurn is a serializable snapshot of one TUI turn.
type PersistedTurn struct {
	ID            int             `json:"id"`
	UserInput     string          `json:"user_input"`
	FinalStatus   int             `json:"final_status"`
	ErrorMsg      string          `json:"error_msg,omitempty"`
	Reply         string          `json:"reply,omitempty"`
	LLMOutput     string          `json:"llm_output,omitempty"`
	CompletionMsg string          `json:"completion_msg,omitempty"`
	ProjectDir    string          `json:"project_dir,omitempty"`
	ChatMode      bool            `json:"chat_mode,omitempty"`
	Tasks         []PersistedTask `json:"tasks,omitempty"`
}

// PersistedTask is a serializable task row.
type PersistedTask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}
