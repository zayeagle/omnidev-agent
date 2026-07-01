package agent

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/commands"
)

// tryBuiltinCommand handles session slash commands without LLM or task decomposition.
func (a *Agent) tryBuiltinCommand(_ context.Context, instruction string, msgCh chan<- tea.Msg) (handled bool, err error) {
	cmd, _, ok := commands.Parse(instruction)
	if !ok {
		return false, nil
	}
	text := commands.HelpText()
	if cmd != "help" {
		// Non-help builtins from RunLoop are unexpected; still avoid the pipeline.
		text = "Built-in command /" + cmd + " is handled by the TUI. Use the interactive client for full output."
	}
	msgCh <- StreamChunkMsg{Content: text, Done: true}
	msgCh <- DoneMsg{}
	return true, nil
}
