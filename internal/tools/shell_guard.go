package tools

import (
	"fmt"
	"regexp"
	"strings"
)

var longRunningShellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bgo\s+run\b`),
	regexp.MustCompile(`(?i)\bnpm\s+(run\s+)?(start|dev|serve)\b`),
	regexp.MustCompile(`(?i)\byarn\s+(start|dev)\b`),
	regexp.MustCompile(`(?i)\bpnpm\s+(run\s+)?(start|dev)\b`),
	regexp.MustCompile(`(?i)\bpython\s+-m\s+http\.server\b`),
	regexp.MustCompile(`(?i)\buvicorn\b`),
	regexp.MustCompile(`(?i)\bflask\s+run\b`),
	regexp.MustCompile(`(?i)\bdjango-admin\s+runserver\b`),
	regexp.MustCompile(`(?i)\bair\b`), // go live reload
}

const longRunningShellHint = "Long-running commands (go run, dev servers, etc.) block the agent. " +
	"Use `go build ./...` and `go test ./...` to verify code; the agent runs build verification automatically when the task finishes."

// LongRunningShellReason returns a user-facing rejection reason when cmd would block indefinitely.
func LongRunningShellReason(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	for _, re := range longRunningShellPatterns {
		if re.MatchString(cmd) {
			return fmt.Sprintf("%s Blocked: %s", longRunningShellHint, truncateCmd(cmd, 120))
		}
	}
	return ""
}

func truncateCmd(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
