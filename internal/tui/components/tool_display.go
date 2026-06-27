package components

import (
	"fmt"
	"strings"
)

// PendingToolName returns the name of the in-flight tool call, if any.
func (t *Turn) PendingToolName() string {
	for i := len(t.ToolCalls) - 1; i >= 0; i-- {
		if t.ToolCalls[i].Status == StatusRunning {
			return t.ToolCalls[i].Name
		}
	}
	if len(t.ToolCalls) > 0 {
		return t.ToolCalls[len(t.ToolCalls)-1].Name
	}
	return ""
}

// SummarizeToolResult returns a short display string for the TUI (not full tool payload).
func SummarizeToolResult(toolName string, success bool, data, errMsg string) string {
	if !success {
		if toolName == "shell_exec" {
			combined := errMsg
			if data != "" {
				combined = errMsg + "\n" + data
			}
			if s := summarizeGoTestOutput(combined); s != "" {
				return s
			}
			if idx := strings.Index(errMsg, "\n"); idx > 0 {
				return truncateDisplay(strings.TrimSpace(errMsg[:idx]), 120)
			}
		}
		return truncateDisplay(errMsg, 120)
	}
	switch toolName {
	case "read_file":
		return summarizeReadFile(data)
	case "list_dir":
		return summarizeListDir(data)
	case "search_file", "search_code":
		return summarizeSearch(data)
	case "shell_exec":
		return summarizeShellOutput(data)
	case "write_file", "edit_file", "delete_file":
		return truncateDisplay(data, 160)
	default:
		return truncateDisplay(data, 120)
	}
}

func summarizeReadFile(data string) string {
	if data == "" {
		return "read empty file (0 bytes)"
	}
	lines := strings.Count(data, "\n") + 1
	return fmt.Sprintf("read %d lines (%d bytes)", lines, len(data))
}

func summarizeListDir(data string) string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return "empty directory"
	}
	entries := strings.Split(trimmed, "\n")
	switch len(entries) {
	case 1:
		return "1 entry: " + entries[0]
	case 2, 3, 4, 5:
		return fmt.Sprintf("%d entries: %s", len(entries), strings.Join(entries, ", "))
	default:
		return fmt.Sprintf("%d entries (%s, …)", len(entries), strings.Join(entries[:3], ", "))
	}
}

func summarizeSearch(data string) string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return "no matches"
	}
	n := strings.Count(trimmed, "\n") + 1
	if n == 1 {
		return "1 match"
	}
	return fmt.Sprintf("%d matches", n)
}

func summarizeShellOutput(data string) string {
	if s := summarizeGoTestOutput(data); s != "" {
		return s
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "(no output)" {
		return "completed (no output)"
	}
	lines := strings.Split(trimmed, "\n")
	first := truncateDisplay(lines[0], 72)
	if len(lines) == 1 {
		return first
	}
	return fmt.Sprintf("%s (+%d lines)", first, len(lines)-1)
}

// summarizeGoTestOutput condenses go test stdout/stderr for the TUI.
func summarizeGoTestOutput(combined string) string {
	if combined == "" {
		return ""
	}
	if !strings.Contains(combined, "PASS:") &&
		!strings.Contains(combined, "FAIL:") &&
		!strings.Contains(combined, "--- FAIL:") &&
		!strings.Contains(combined, "ok  \t") &&
		!strings.Contains(combined, "ok \t") {
		return ""
	}
	pass := strings.Count(combined, "PASS:")
	fail := strings.Count(combined, "FAIL:")
	if strings.Contains(combined, "--- FAIL:") && fail == 0 {
		fail = strings.Count(combined, "--- FAIL:")
	}
	if fail > 0 {
		if pass > 0 {
			return fmt.Sprintf("tests failed (%d passed, %d failed)", pass, fail)
		}
		return fmt.Sprintf("tests failed (%d failed)", fail)
	}
	if pass > 0 {
		return fmt.Sprintf("all tests passed (%d)", pass)
	}
	if strings.Contains(combined, "ok  \t") || strings.Contains(combined, "ok \t") {
		return "all tests passed"
	}
	return ""
}

func truncateDisplay(s string, max int) string {
	s = strings.TrimSpace(s)
	if max < 8 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
