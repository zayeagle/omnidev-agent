package tui

import (
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

func snapshotUI(turns *components.TurnList, turnCount int, outputDir string) *session.PersistedUI {
	if turns == nil || turns.Count() == 0 {
		return &session.PersistedUI{TurnCount: turnCount, OutputDir: outputDir}
	}
	ui := &session.PersistedUI{
		TurnCount: turnCount,
		OutputDir: outputDir,
	}
	for _, t := range turns.AllTurns() {
		if t == nil {
			continue
		}
		pt := session.PersistedTurn{
			ID:            t.ID,
			UserInput:     t.UserInput,
			FinalStatus:   int(t.FinalStatus),
			ErrorMsg:      t.ErrorMsg,
			Reply:         t.ReplyText(),
			LLMOutput:     t.LLMText(),
			CompletionMsg: t.CompletionText(),
			ProjectDir:    t.ProjectDirText(),
			ChatMode:      t.IsChatMode(),
		}
		for _, tk := range t.Tasks {
			if tk == nil {
				continue
			}
			pt.Tasks = append(pt.Tasks, session.PersistedTask{
				ID:          tk.ID,
				Description: tk.Description,
				Status:      int(tk.Status),
			})
		}
		ui.Turns = append(ui.Turns, pt)
	}
	return ui
}

func restoreUI(turns *components.TurnList, ui *session.PersistedUI) int {
	if turns == nil || ui == nil || len(ui.Turns) == 0 {
		return 0
	}
	maxID := 0
	for _, pt := range ui.Turns {
		t := turns.AddTurn(pt.ID, pt.UserInput)
		t.SetChatMode(pt.ChatMode)
		t.SetActive(false)
		t.FinalStatus = components.TurnFinalStatus(pt.FinalStatus)
		t.ErrorMsg = pt.ErrorMsg
		if pt.Reply != "" {
			t.RestoreReply(pt.Reply)
		}
		if pt.LLMOutput != "" {
			t.RestoreLLM(pt.LLMOutput)
		}
		if pt.ProjectDir != "" || pt.CompletionMsg != "" {
			t.SetCompletion(pt.CompletionMsg, pt.ProjectDir)
		}
		for _, tk := range pt.Tasks {
			t.AddOrUpdateTask(tk.ID, tk.Description, components.ItemStatus(tk.Status))
		}
		t.RecomputeTaskBlocked()
		if pt.ID > maxID {
			maxID = pt.ID
		}
	}
	turns.ScrollToBottom()
	if ui.TurnCount > maxID {
		return ui.TurnCount
	}
	return maxID
}

func isAgentPromptInput(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" || input == "quit" || input == "exit" {
		return false
	}
	return !isSessionSlashCommand(input)
}

func promptHistoryFromTurns(turns *components.TurnList) []string {
	if turns == nil {
		return nil
	}
	var out []string
	for _, t := range turns.AllTurns() {
		if t == nil {
			continue
		}
		if isAgentPromptInput(t.UserInput) {
			out = append(out, strings.TrimSpace(t.UserInput))
		}
	}
	return out
}

func restoreInputHistory(input *components.InputLine, turns *components.TurnList) {
	if input == nil {
		return
	}
	input.LoadHistory(promptHistoryFromTurns(turns))
}

func hydrateTurnsFromEntries(turns *components.TurnList, entries []session.Entry) int {
	if turns == nil || len(entries) == 0 {
		return 0
	}
	turnCount := 0
	var current *components.Turn
	flush := func() {
		if current == nil {
			return
		}
		current.SetActive(false)
		if current.FinalStatus == components.TurnRunning {
			current.MarkDone()
		}
	}
	for _, e := range entries {
		switch e.Role {
		case "user":
			content := strings.TrimSpace(e.Content)
			if content == "" || strings.HasPrefix(content, "[") {
				continue
			}
			flush()
			turnCount++
			current = turns.AddTurn(turnCount, content)
			current.SetActive(false)
		case "assistant":
			if current != nil && strings.TrimSpace(e.Content) != "" {
				current.RestoreReply(e.Content)
			}
		}
	}
	flush()
	turns.ScrollToBottom()
	return turnCount
}
