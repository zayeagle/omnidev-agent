package agent

import (
	"fmt"
	"strings"
)

// FormatCompletionSummary builds a short user-facing completion message.
func FormatCompletionSummary(taskCount int, projectDir string) string {
	var b strings.Builder
	if taskCount > 1 {
		b.WriteString(fmt.Sprintf("All %d tasks completed.", taskCount))
	} else {
		b.WriteString("Task completed.")
	}
	if projectDir != "" {
		b.WriteString("\n\nProject location:\n  ")
		b.WriteString(projectDir)
	}
	return b.String()
}

// NewAllComplete builds a completion message for the TUI.
func NewAllComplete(taskCount int, projectDir string) AllCompleteMsg {
	return AllCompleteMsg{
		Summary:    FormatCompletionSummary(taskCount, projectDir),
		ProjectDir: projectDir,
	}
}
