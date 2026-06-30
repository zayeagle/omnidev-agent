package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

const reviewNudgeText = `[REVIEW INCOMPLETE] You stopped too early. The user asked to analyze whether existing code is complete — do NOT finish after reading only a few small files.

Required before your final analysis:
1. list_dir the project root and main packages (cmd/, internal/, or equivalent)
2. read_file the entry point, game loop, input, rendering, and collision/score logic
3. search_code for TODO/FIXME/unimplemented if needed
4. Then write a structured conclusion: features done / missing / quality / build readiness

Continue with tools until you can answer comprehensively, then reply with your full analysis (no more tool calls).`

const synthesizeConclusionSystem = `You write the final user-facing conclusion for a coding agent session.
Respond in the same language as the user's request (Chinese if they wrote Chinese).
Be specific and actionable. Cover: whether the request was fulfilled, code completeness, build/binary status, and gaps.
Do not include file paths for the project root (shown separately). No markdown headers — plain paragraphs and bullet lines are fine.`

// looksLikeReviewInstruction detects analyze/review/check completeness requests.
func looksLikeReviewInstruction(instruction string) bool {
	lower := strings.ToLower(strings.TrimSpace(instruction))
	if lower == "" {
		return false
	}
	reviewHints := []string{
		"check", "review", "analyze", "analysis", "audit", "assess", "evaluate",
		"complete", "completeness", "finished", "ready", "完善", "检查", "分析",
		"评估", "审查", "是否完成", "完成了吗", "实现了", "功能", "齐全",
	}
	for _, h := range reviewHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return looksLikeTaskQuestion(instruction) && (strings.Contains(lower, "?") || strings.Contains(instruction, "？"))
}

func reviewSystemAddendum(instruction string) string {
	if !looksLikeReviewInstruction(instruction) {
		return ""
	}
	return `

REVIEW MODE: The user wants analysis of EXISTING code — not a quick skim.
Explore thoroughly (list_dir, read entry/game logic, search TODOs) before your final answer.
When exploration is done, write a clear structured conclusion as assistant text (then stop calling tools).`
}

func countExplorationTools(entries []session.Entry) (reads, lists, searches int) {
	for _, e := range entries {
		for _, tc := range e.AssistantToolCalls {
			switch tc.Name {
			case "read_file":
				reads++
			case "list_dir":
				lists++
			case "search_code", "search_file":
				searches++
			}
		}
	}
	return reads, lists, searches
}

func hasSubstantialConclusionText(text string) bool {
	text = strings.TrimSpace(text)
	if len([]rune(text)) >= 120 {
		return true
	}
	lower := strings.ToLower(text)
	markers := []string{
		"missing", "complete", "implement", "conclusion", "summary", "gap", "pass", "fail",
		"缺失", "完善", "结论", "总结", "未实现", "已实现", "功能", "编译", "binary",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func needsMoreReview(instruction string, sess *session.Session) bool {
	if !looksLikeReviewInstruction(instruction) {
		return false
	}
	if hasSubstantialConclusionText(latestAssistantText(sess)) {
		return false
	}
	reads, lists, _ := countExplorationTools(sess.EntriesCopy())
	uniquePaths := countUniqueReadPaths(sess.EntriesCopy())
	if lists == 0 {
		return true
	}
	if uniquePaths < 4 && reads < 6 {
		return true
	}
	return false
}

func latestAssistantText(sess *session.Session) string {
	entries := sess.EntriesCopy()
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Role == "assistant" && strings.TrimSpace(entries[i].Content) != "" {
			return entries[i].Content
		}
	}
	return ""
}

func latestUserInstruction(sess *session.Session) string {
	entries := sess.EntriesCopy()
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Role == "user" {
			return entries[i].Content
		}
	}
	return ""
}

func extractConclusionFromSession(sess *session.Session, results []TaskResult) string {
	var parts []string
	for _, r := range results {
		if c := strings.TrimSpace(r.Content); c != "" {
			parts = append(parts, c)
		}
	}
	if sess != nil {
		entries := sess.EntriesCopy()
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			if e.Role == "assistant" && strings.TrimSpace(e.Content) != "" {
				parts = append(parts, e.Content)
				break
			}
		}
		for _, e := range entries {
			if e.Role != "system" {
				continue
			}
			if strings.Contains(e.Content, "[VERIFICATION") {
				parts = append(parts, strings.TrimSpace(e.Content))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// BuildFinalConclusion produces the user-facing completion text shown above the project path.
func (a *Agent) BuildFinalConclusion(ctx context.Context, results []TaskResult, criteria []CriterionStatus) string {
	if a.acceptanceStrict && len(criteria) > 0 && allCriteriaMet(criteria) {
		// Detailed acceptance report is shown in the collapsible TUI panel, not duplicated here.
		if extracted := extractConclusionFromSession(a.session, results); hasSubstantialConclusionText(extracted) {
			return trimConclusion(extracted)
		}
		return ""
	}
	instruction := latestUserInstruction(a.session)
	if extracted := extractConclusionFromSession(a.session, results); hasSubstantialConclusionText(extracted) {
		return trimConclusion(extracted)
	}
	if a.provider == nil {
		return fallbackConclusion(instruction, results)
	}
	synthesized, err := a.synthesizeConclusion(ctx, instruction, results)
	if err != nil || strings.TrimSpace(synthesized) == "" {
		return fallbackConclusion(instruction, results)
	}
	return trimConclusion(synthesized)
}

func formatConclusionFromCriteria(statuses []CriterionStatus) string {
	var b strings.Builder
	for _, s := range statuses {
		mark := "[x]"
		if !s.Met {
			mark = "[ ]"
		}
		b.WriteString(fmt.Sprintf("%s %s", mark, s.Text))
		if s.Evidence != "" {
			b.WriteString(" — ")
			b.WriteString(s.Evidence)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (a *Agent) synthesizeConclusion(ctx context.Context, instruction string, results []TaskResult) (string, error) {
	digest := buildSessionDigest(a.session, results)
	prompt := fmt.Sprintf("User request:\n%s\n\nSession activity and notes:\n%s\n\nWrite the final conclusion for the user.", instruction, digest)
	messages := []llm.Message{
		{Role: "system", Content: synthesizeConclusionSystem},
		{Role: "user", Content: prompt},
	}
	resp, err := stream.RetryChat(ctx, a.provider, &llm.Request{Messages: messages}, a.retryConfig)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func buildSessionDigest(sess *session.Session, results []TaskResult) string {
	var b strings.Builder
	for _, r := range results {
		if r.Content != "" {
			b.WriteString(fmt.Sprintf("- sub-task %s: %s\n", r.TaskID, truncateForDigest(r.Content, 800)))
		}
		if r.Error != "" {
			b.WriteString(fmt.Sprintf("- sub-task %s error: %s\n", r.TaskID, r.Error))
		}
	}
	for _, e := range sess.EntriesCopy() {
		switch e.Role {
		case "user":
			b.WriteString("user: " + truncateForDigest(e.Content, 400) + "\n")
		case "assistant":
			if e.Content != "" {
				b.WriteString("assistant: " + truncateForDigest(e.Content, 600) + "\n")
			}
			for _, tc := range e.AssistantToolCalls {
				b.WriteString(fmt.Sprintf("tool_call: %s\n", tc.Name))
			}
		case "system":
			if strings.Contains(e.Content, "[VERIFICATION") || strings.Contains(e.Content, "[SUB-TASK") {
				b.WriteString(truncateForDigest(e.Content, 500) + "\n")
			}
		}
	}
	return b.String()
}

func truncateForDigest(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

func fallbackConclusion(instruction string, results []TaskResult) string {
	if c := extractConclusionFromSession(nil, results); c != "" {
		return trimConclusion(c)
	}
	if looksLikeReviewInstruction(instruction) {
		return "Review finished, but the agent did not produce a detailed analysis. Expand Thinking or re-run with a narrower checklist (e.g. list missing snake-game features and run go build -o bin/snake.exe)."
	}
	return "Task finished. See tool steps above for details."
}

func trimConclusion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Task completed.")
	s = strings.TrimPrefix(s, "All tasks completed.")
	return strings.TrimSpace(s)
}
