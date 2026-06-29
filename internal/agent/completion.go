package agent

// NewAllComplete builds a completion message for the TUI.
// Summary is the user-facing conclusion; ProjectDir is shown below it in the completion panel.
func NewAllComplete(conclusion, projectDir string) AllCompleteMsg {
	return AllCompleteMsg{
		Summary:    conclusion,
		ProjectDir: projectDir,
	}
}
