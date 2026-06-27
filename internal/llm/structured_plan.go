package llm

import (
	"encoding/json"
	"strings"
)

// StructuredPlan is a multi-step plan embedded in LLM text output.
// Each step is a tool call, reasoning block, or final output.
type StructuredPlan struct {
	Steps []Step `json:"steps"`
}

// Step is one action in a StructuredPlan.
type Step struct {
	ID     string         `json:"id"`
	Action string         `json:"action"` // "tool_call" | "reasoning" | "output"
	Tool   string         `json:"tool,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Text   string         `json:"text,omitempty"`
}

// ParseStructuredPlan extracts a StructuredPlan from LLM response text.
// Looks for a ```json ... ``` block or bare JSON object. Returns nil on failure.
func ParseStructuredPlan(content string) *StructuredPlan {
	if content == "" {
		return nil
	}

	jsonText := extractJSONBlock(content)
	if jsonText == "" {
		return nil
	}

	var plan StructuredPlan
	if err := json.Unmarshal([]byte(jsonText), &plan); err != nil {
		return nil
	}
	if len(plan.Steps) == 0 {
		return nil
	}
	return &plan
}

func extractJSONBlock(text string) string {
	if start := findJSONBlockStart(text); start >= 0 {
		if end := findJSONBlockEnd(text, start); end > start {
			return text[start:end]
		}
	}
	return findBareJSON(text)
}

func findJSONBlockStart(text string) int {
	if i := strings.Index(text, "```json"); i >= 0 {
		j := i + 7
		for j < len(text) && (text[j] == '\n' || text[j] == '\r') {
			j++
		}
		return j
	}
	return -1
}

func findJSONBlockEnd(text string, start int) int {
	end := strings.Index(text[start:], "```")
	if end < 0 {
		return -1
	}
	return start + end
}

func findBareJSON(text string) string {
	i := strings.IndexAny(text, "{[")
	if i < 0 {
		return ""
	}

	open := text[i]
	close := byte('}')
	if open == '[' {
		close = ']'
	}

	depth := 0
	for j := i; j < len(text); j++ {
		if text[j] == open {
			depth++
		} else if text[j] == close {
			depth--
			if depth == 0 {
				return text[i : j+1]
			}
		}
	}
	return ""
}

// ExtractToolCallsFromPlan converts tool_call steps into ToolCall entries.
func ExtractToolCallsFromPlan(plan *StructuredPlan) []ToolCall {
	if plan == nil {
		return nil
	}
	var calls []ToolCall
	for _, step := range plan.Steps {
		if step.Action == "tool_call" && step.Tool != "" {
			calls = append(calls, ToolCall{
				ID:        step.ID,
				Name:      step.Tool,
				Arguments: step.Args,
			})
		}
	}
	return calls
}

// ExtractReasoningText concatenates reasoning steps for TUI display.
func ExtractReasoningText(plan *StructuredPlan) string {
	if plan == nil {
		return ""
	}
	var parts []string
	for _, step := range plan.Steps {
		if step.Action == "reasoning" && step.Text != "" {
			parts = append(parts, step.Text)
		}
	}
	return strings.Join(parts, "\n")
}
