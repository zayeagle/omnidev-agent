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
Structure the summary with these sections (plain labels, no markdown headers):
1. Changes & optimizations — what files/features were changed and what was improved in this session
2. Next steps — concrete directions to optimize, extend, or harden the work further
Be specific and actionable. Do not repeat the full acceptance checklist (shown separately). No markdown headers.`

const synthesizePartialSystem = `You write a session summary when a coding agent stopped before fully finishing.
Respond in the same language as the user's request (Chinese if they wrote Chinese).
Structure the summary with these sections (plain labels, no markdown headers):
1. Failure reason — why the task did not complete (be specific)
2. Recommended solution — concrete steps to fix the issue and retry successfully
3. Partial progress — what was already accomplished (if any)
4. Next steps — optional follow-up optimizations after unblocking
Be honest about partial progress. No markdown headers.`

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
	return a.BuildSessionSummary(ctx, SessionOutcomeSuccess, "", results, criteria)
}

// BuildSessionSummary synthesizes a user-facing summary for success, failure, or iteration-limit stops.
func (a *Agent) BuildSessionSummary(ctx context.Context, outcome SessionOutcome, stopReason string, results []TaskResult, criteria []CriterionStatus) string {
	instruction := latestUserInstruction(a.session)
	if outcome == SessionOutcomeSuccess && a.acceptanceStrict && len(criteria) > 0 && allCriteriaMet(criteria) {
		if extracted := extractConclusionFromSession(a.session, results); hasSubstantialConclusionText(extracted) {
			return trimConclusion(extracted)
		}
		return formatSuccessFallbackSummary(instruction, results, criteria)
	}
	if extracted := extractConclusionFromSession(a.session, results); hasSubstantialConclusionText(extracted) && outcome == SessionOutcomeSuccess {
		return trimConclusion(extracted)
	}
	if a.provider == nil {
		return fallbackSessionSummary(outcome, stopReason, instruction, results, criteria)
	}
	synthesized, err := a.synthesizeSessionSummary(ctx, outcome, stopReason, instruction, results, criteria)
	if err != nil || strings.TrimSpace(synthesized) == "" {
		return fallbackSessionSummary(outcome, stopReason, instruction, results, criteria)
	}
	return trimConclusion(synthesized)
}

// SessionOutcome classifies how the agent loop ended for summary generation.
type SessionOutcome string

const (
	SessionOutcomeSuccess SessionOutcome = "success"
	SessionOutcomePartial SessionOutcome = "partial"
	SessionOutcomeFailed  SessionOutcome = "failed"
)

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
	return a.synthesizeSessionSummary(ctx, SessionOutcomeSuccess, "", instruction, results, nil)
}

func (a *Agent) synthesizeSessionSummary(ctx context.Context, outcome SessionOutcome, stopReason, instruction string, results []TaskResult, criteria []CriterionStatus) (string, error) {
	digest := buildSessionDigest(a.session, results)
	if len(criteria) > 0 {
		digest += "\n\nAcceptance criteria:\n" + formatConclusionFromCriteria(criteria)
	}
	prompt := fmt.Sprintf("User request:\n%s\n\nOutcome: %s\n", instruction, outcome)
	if stopReason != "" {
		prompt += fmt.Sprintf("Stop reason: %s\n", stopReason)
	}
	prompt += fmt.Sprintf("\nSession activity and notes:\n%s\n\nWrite the summary for the user.", digest)
	system := synthesizeConclusionSystem
	if outcome != SessionOutcomeSuccess {
		system = synthesizePartialSystem
	}
	messages := []llm.Message{
		{Role: "system", Content: system},
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
	return fallbackSessionSummary(SessionOutcomeSuccess, "", instruction, results, nil)
}

func fallbackSessionSummary(outcome SessionOutcome, stopReason, instruction string, results []TaskResult, criteria []CriterionStatus) string {
	if c := extractConclusionFromSession(nil, results); c != "" && outcome == SessionOutcomeSuccess {
		return trimConclusion(c)
	}
	switch outcome {
	case SessionOutcomeSuccess:
		return formatSuccessFallbackSummary(instruction, results, criteria)
	case SessionOutcomeFailed:
		return formatFailedFallbackSummary(stopReason, instruction, results, criteria)
	default:
		return formatPartialFallbackSummary(stopReason, instruction, criteria)
	}
}

func formatSuccessFallbackSummary(instruction string, results []TaskResult, criteria []CriterionStatus) string {
	var b strings.Builder
	b.WriteString("Changes & optimizations:\n")
	if len(results) > 0 {
		for _, r := range results {
			if r.Success {
				line := strings.TrimSpace(r.Content)
				if line == "" {
					line = "sub-task " + r.TaskID + " completed"
				}
				b.WriteString("- " + truncateForDigest(line, 200) + "\n")
			}
		}
	} else {
		b.WriteString("- Request addressed: " + truncateForDigest(instruction, 200) + "\n")
	}
	if len(criteria) > 0 {
		b.WriteString("- Acceptance: " + formatVerificationSummary(criteria) + "\n")
	}
	b.WriteString("\nNext steps:\n")
	b.WriteString("- Add or extend automated tests and CI checks\n")
	b.WriteString("- Harden error handling and edge cases\n")
	b.WriteString("- Refine UX and documentation before release\n")
	return strings.TrimSpace(b.String())
}

func formatFailedFallbackSummary(stopReason, instruction string, results []TaskResult, criteria []CriterionStatus) string {
	var b strings.Builder
	reason := strings.TrimSpace(stopReason)
	if reason == "" {
		reason = "Task did not complete."
	}
	b.WriteString("Failure reason:\n")
	b.WriteString(reason + "\n")
	if len(criteria) > 0 {
		b.WriteString("\n" + formatVerificationSummary(criteria) + "\n")
	}
	b.WriteString("\nRecommended solution:\n")
	b.WriteString(failureRecoveryHints(reason, instruction, criteria))
	if len(results) > 0 {
		b.WriteString("\nPartial progress:\n")
		for _, r := range results {
			if r.Success && strings.TrimSpace(r.Content) != "" {
				b.WriteString("- " + truncateForDigest(r.Content, 160) + "\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func formatPartialFallbackSummary(stopReason, instruction string, criteria []CriterionStatus) string {
	var b strings.Builder
	if stopReason != "" {
		b.WriteString("Failure reason:\n")
		b.WriteString(stopReason + "\n")
	}
	b.WriteString("\nRecommended solution:\n")
	b.WriteString("- Send a follow-up message to continue from the saved checkpoint\n")
	if looksLikeReviewInstruction(instruction) {
		b.WriteString("- Narrow the review scope and re-run with a explicit checklist\n")
	}
	if len(criteria) > 0 {
		b.WriteString("\nPartial progress:\n")
		b.WriteString(formatVerificationSummary(criteria) + "\n")
	}
	return strings.TrimSpace(b.String())
}

func failureRecoveryHints(reason, instruction string, criteria []CriterionStatus) string {
	lower := strings.ToLower(reason + " " + instruction)
	var hints []string
	switch {
	case strings.Contains(lower, "network") || strings.Contains(lower, "unreachable"):
		hints = append(hints, "- Check API key, base URL, and network connectivity; retry when online")
	case strings.Contains(lower, "build") || strings.Contains(lower, "compile"):
		hints = append(hints, "- Run go build ./... locally, fix compile errors, then resume")
	case strings.Contains(lower, "acceptance") || strings.Contains(lower, "criteria"):
		hints = append(hints, "- Review failed acceptance rows above; fix gaps and resume from checkpoint")
	case strings.Contains(lower, "sub-task"):
		hints = append(hints, "- Inspect sub-task errors in the transcript; fix blockers and retry failed tasks")
	default:
		hints = append(hints, "- Review tool output above, fix the blocking issue, then send continue or a follow-up")
	}
	if len(criteria) > 0 && !allCriteriaMet(criteria) {
		hints = append(hints, "- Address unmet acceptance criteria before marking the task done")
	}
	hints = append(hints, "- Use Ctrl+C to interrupt, then follow up to resume without losing progress")
	return strings.Join(hints, "\n")
}

func trimConclusion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Task completed.")
	s = strings.TrimPrefix(s, "All tasks completed.")
	return strings.TrimSpace(s)
}
