package agent

import (
	"strings"
)

// FollowUpMode decides how to handle a message after Ctrl+C interrupt.
type FollowUpMode int

const (
	FollowUpUnknown FollowUpMode = iota
	FollowUpContinue
	FollowUpReplanLight
	FollowUpReplanFull
)

func (m FollowUpMode) String() string {
	switch m {
	case FollowUpContinue:
		return "continue"
	case FollowUpReplanLight:
		return "replan_light"
	case FollowUpReplanFull:
		return "replan_full"
	default:
		return "unknown"
	}
}

// ClassifyFollowUpIntent picks resume vs light/full re-plan from the user's new message.
func ClassifyFollowUpIntent(followUp string) FollowUpMode {
	s := strings.TrimSpace(followUp)
	if s == "" {
		return FollowUpContinue
	}
	lower := strings.ToLower(s)

	if matchesAny(lower, forceReplanPhrases) {
		return FollowUpReplanFull
	}
	if isContinuePhrase(s, lower) {
		return FollowUpContinue
	}
	if matchesAny(lower, newDirectionPhrases) {
		return FollowUpReplanFull
	}
	if matchesAny(lower, supplementPhrases) || looksLikeCodeMod(s) {
		return FollowUpReplanLight
	}
	if len([]rune(s)) <= 16 {
		return FollowUpContinue
	}
	return FollowUpReplanLight
}

func isContinuePhrase(s, lower string) bool {
	if len([]rune(s)) > 24 {
		return false
	}
	exact := []string{
		"continue", "resume", "go on", "keep going", "proceed", "carry on",
		"继续", "接着", "接着做", "继续做", "继续吧", "往下做", "go ahead",
	}
	for _, p := range exact {
		if lower == p || strings.HasPrefix(lower, p+" ") || strings.HasPrefix(lower, p+",") {
			return true
		}
	}
	return false
}

func matchesAny(lower string, phrases []string) bool {
	for _, p := range phrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

var forceReplanPhrases = []string{
	"重新规划", "重规划", "重新分解", "重做计划", "重新计划",
	"replan", "re-plan", "re plan", "new plan", "redesign tasks", "redecompose",
}

var newDirectionPhrases = []string{
	"instead of", "rather than", "switch to", "stop doing", "forget about",
	"don't do", "do not do", "abandon", "change direction", "pivot to",
	"改成", "改为", "别做", "不要做了", "换成", "不做", "放弃", "改做", "转向",
}

var supplementPhrases = []string{
	"also ", "additionally", "add ", "and add", "plus ", "as well",
	"另外", "还要", "补充", "再加", "顺便", "同时", "以及", "并且",
}

func followUpModeLabel(mode FollowUpMode) string {
	switch mode {
	case FollowUpContinue:
		return "Resuming previous task plan."
	case FollowUpReplanLight:
		return "Re-planning tasks (adjusting plan, preserving completed work)."
	case FollowUpReplanFull:
		return "Re-planning tasks (fresh decomposition)."
	default:
		return "Resuming previous task plan."
	}
}

func formatCheckpointProgress(cp *Checkpoint) string {
	if cp == nil || len(cp.Tasks) == 0 {
		return "(no prior sub-tasks)"
	}
	completed := cp.CompletedTaskIDs()
	var b strings.Builder
	for _, t := range cp.Tasks {
		status := "pending"
		if completed[t.ID] {
			status = "completed"
			for _, r := range cp.Results {
				if r.TaskID == t.ID && !r.Success {
					status = "failed"
					break
				}
			}
		}
		b.WriteString("- [")
		b.WriteString(t.ID)
		b.WriteString("] ")
		b.WriteString(t.Description)
		b.WriteString(": ")
		b.WriteString(status)
		b.WriteRune('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func filterResultsForTasks(results []TaskResult, tasks []Task) []TaskResult {
	if len(results) == 0 || len(tasks) == 0 {
		return nil
	}
	ids := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		ids[t.ID] = true
	}
	out := make([]TaskResult, 0, len(results))
	for _, r := range results {
		if ids[r.TaskID] && r.Success {
			out = append(out, r)
		}
	}
	return out
}
