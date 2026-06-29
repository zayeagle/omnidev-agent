package agent

import (
	"fmt"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
)

const (
	defaultToolResultsKeepFull = 3
	defaultContextMinKeep      = 10
	defaultGuardAnalysisMax    = 4000
)

// ContextSlimOptions controls how much history is sent to the LLM each turn.
type ContextSlimOptions struct {
	ToolResultsKeepFull int // last N tool results at full size; older ones become refs
	MinKeepEntries      int // entries kept verbatim during compaction
	GuardAnalysisMax    int // max chars for [PROJECT ANALYSIS] in session
}

func DefaultContextSlimOptions() ContextSlimOptions {
	return ContextSlimOptions{
		ToolResultsKeepFull: defaultToolResultsKeepFull,
		MinKeepEntries:      defaultContextMinKeep,
		GuardAnalysisMax:    defaultGuardAnalysisMax,
	}
}

// PipelineOptions toggles optional pre-loop LLM stages (off = heuristic / skip).
type PipelineOptions struct {
	UseLLMClassifier   bool
	UseLLMRequirements bool
	UseLLMComplexity   bool
	PlanMode           int // 0=auto LLM decides 1 vs N (default), 1=same as 0, 2=never LLM (single task)
}

func DefaultPipelineOptions() PipelineOptions {
	return PipelineOptions{
		UseLLMClassifier:   false,
		UseLLMRequirements: false,
		UseLLMComplexity:   false,
		PlanMode:           0,
	}
}

// SlimToolArguments removes bulky payload fields from stored/sent tool call args.
func SlimToolArguments(name string, args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}
	out := make(map[string]interface{}, len(args))
	for k, v := range args {
		out[k] = v
	}
	switch name {
	case "write_file":
		if content, ok := out["content"].(string); ok && len(content) > 0 {
			lines := strings.Count(content, "\n") + 1
			out["content"] = fmt.Sprintf("[omitted %d chars, %d lines — written via tool]", len(content), lines)
		}
	case "edit_file":
		if oldS, ok := out["old_snippet"].(string); ok && len(oldS) > 80 {
			out["old_snippet"] = truncateForRef(oldS, 80)
		}
		if newS, ok := out["new_snippet"].(string); ok && len(newS) > 80 {
			out["new_snippet"] = truncateForRef(newS, 80)
		}
	}
	return out
}

func truncateForRef(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("… [%d chars omitted]", len(s)-max)
}

// SlimToolResultForHistory replaces an old tool result with a one-line continuation handle.
func SlimToolResultForHistory(name, result string) string {
	ref := extractPartialRef(result)
	if ref != "" {
		return fmt.Sprintf("[archived %s result — reload: %s]", name, ref)
	}
	if path := extractPathArg(result); path != "" {
		return fmt.Sprintf("[archived %s result — use read_file on %q]", name, path)
	}
	preview := strings.TrimSpace(result)
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	if preview == "" {
		preview = "(empty)"
	}
	return fmt.Sprintf("[archived %s: %s]", name, preview)
}

func extractPartialRef(result string) string {
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[PARTIAL ") && strings.Contains(line, "full:") {
			if i := strings.Index(line, "full:"); i >= 0 {
				return strings.TrimSpace(line[i+len("full:"):])
			}
		}
		if strings.HasPrefix(line, "Continue:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Continue:"))
		}
	}
	return ""
}

func extractPathArg(s string) string {
	if i := strings.Index(s, "path="); i >= 0 {
		rest := s[i+5:]
		if j := strings.IndexAny(rest, `" )`); j > 0 {
			return rest[1 : j-1]
		}
	}
	return ""
}

// CompressGuardAnalysis keeps guard scan output within budget for session storage.
func CompressGuardAnalysis(raw string, maxChars int) string {
	if maxChars <= 0 || len(raw) <= maxChars {
		return raw
	}
	var sb strings.Builder
	sb.WriteString("[PROJECT ANALYSIS — condensed for context budget]\n")
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "[PROJECT ANALYSIS]" {
			continue
		}
		if strings.HasPrefix(line, "[PARTIAL ") {
			sb.WriteString(line + "\n")
			continue
		}
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		sb.WriteString(line + "\n")
		if sb.Len() >= maxChars-120 {
			break
		}
	}
	sb.WriteString("\n(Full scan details available via list_dir / read_file / search_code.)\n")
	out := sb.String()
	if len(out) > maxChars {
		return out[:maxChars] + "\n… [analysis truncated — use tools to explore]"
	}
	return out
}

// toolEntryRecency maps entry index → 0 = newest tool result, 1 = second newest, etc.
func toolEntryRecency(entries []session.Entry) map[int]int {
	var toolIdx []int
	for i, e := range entries {
		if e.Role == "tool" {
			toolIdx = append(toolIdx, i)
		}
	}
	recency := make(map[int]int, len(toolIdx))
	for rank, idx := range toolIdx {
		recency[idx] = len(toolIdx) - 1 - rank
	}
	return recency
}

func estimateEntryTokens(e session.Entry) int {
	if e.Role == "tool" && len(e.ToolCalls) > 0 {
		n := 0
		for _, tc := range e.ToolCalls {
			n += runeLen(tc.Name) + runeLen(tc.Result) + runeLen(tc.Error)
		}
		return n/3 + 1
	}
	n := runeLen(e.Content)
	for _, tc := range e.AssistantToolCalls {
		n += runeLen(tc.Name)
		for _, v := range tc.Arguments {
			n += runeLen(fmt.Sprint(v))
		}
	}
	for _, tc := range e.ToolCalls {
		n += runeLen(tc.Name) + runeLen(tc.Result) + runeLen(tc.Error)
	}
	return n/3 + 1
}

func runeLen(s string) int {
	return len([]rune(s))
}

// ParentContextForSubAgent extracts slim shared context for sub-agents.
func ParentContextForSubAgent(entries []session.Entry) string {
	var parts []string
	for _, e := range entries {
		if e.Role != "system" {
			continue
		}
		c := e.Content
		switch {
		case strings.Contains(c, "[PROJECT ANALYSIS]"):
			parts = append(parts, CompressGuardAnalysis(c, 2500))
		case strings.Contains(c, "[EARLY CONTEXT SUMMARY]"):
			parts = append(parts, truncateForRef(c, 1500))
		case strings.Contains(c, "Requirements analysis:"):
			parts = append(parts, truncateForRef(c, 800))
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Role == "user" {
			parts = append(parts, "Parent user request: "+truncateForRef(entries[i].Content, 500))
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func slimLLMToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	out := make([]llm.ToolCall, len(calls))
	for i, tc := range calls {
		out[i] = llm.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: SlimToolArguments(tc.Name, tc.Arguments),
		}
	}
	return out
}

func slimSessionToolCalls(calls []session.ToolCallData) []session.ToolCallData {
	out := make([]session.ToolCallData, len(calls))
	for i, tc := range calls {
		out[i] = session.ToolCallData{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: SlimToolArguments(tc.Name, tc.Arguments),
		}
	}
	return out
}

func slimEntryForSummary(e session.Entry) string {
	switch e.Role {
	case "tool":
		if len(e.ToolCalls) > 0 {
			tc := e.ToolCalls[0]
			body := tc.Result
			if tc.Error != "" {
				body = tc.Error
			}
			return SlimToolResultForHistory(tc.Name, body)
		}
		return SlimToolResultForHistory("tool", e.Content)
	case "assistant":
		if len(e.AssistantToolCalls) > 0 {
			var names []string
			for _, tc := range e.AssistantToolCalls {
				names = append(names, tc.Name)
			}
			s := e.Content
			if s != "" {
				s = truncateForRef(s, 200) + " | "
			}
			return s + "tool_calls: " + strings.Join(names, ", ")
		}
	}
	c := e.Content
	if e.Role == "system" && strings.Contains(c, "[PROJECT ANALYSIS]") {
		return CompressGuardAnalysis(c, 2000)
	}
	return truncateForRef(c, 600)
}

func toolSummaryLine(toolName string, success bool) string {
	status := "ok"
	if !success {
		status = "error"
	}
	return fmt.Sprintf("[%s %s — see tool_calls result]", toolName, status)
}
