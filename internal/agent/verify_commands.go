package agent

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// RunVerifyCommands executes user/plan-defined shell commands in dir (non-interactive).
func RunVerifyCommands(ctx context.Context, dir string, commands []string) (summary string, ok bool) {
	dir = strings.TrimSpace(dir)
	if dir == "" || len(commands) == 0 {
		return "", true
	}

	var lines []string
	allOK := true
	for _, raw := range commands {
		cmdline := strings.TrimSpace(raw)
		if cmdline == "" {
			continue
		}
		out, err := runShellVerify(ctx, dir, cmdline)
		if err != nil {
			allOK = false
			lines = append(lines, fmt.Sprintf("Custom verify failed (%s):", cmdline))
			lines = append(lines, indentOutput(out, err))
		} else {
			lines = append(lines, fmt.Sprintf("Custom verify passed: %s", cmdline))
		}
	}
	if len(lines) == 0 {
		return "", true
	}
	return strings.Join(lines, "\n"), allOK
}

func runShellVerify(ctx context.Context, dir, cmdline string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdline)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdline)
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil && text == "" {
		text = err.Error()
	}
	return text, err
}

// parseVerifyCommandsFromInstruction extracts explicit verify commands from user text.
// Supported forms: "verify: go test ...", "验收: pytest", "run `go test -run Foo`".
func parseVerifyCommandsFromInstruction(instruction string) []string {
	var cmds []string
	seen := make(map[string]bool)
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		cmds = append(cmds, c)
	}

	for _, line := range strings.Split(instruction, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "verify:"):
			add(trimmed[7:])
		case strings.HasPrefix(lower, "验收:") || strings.HasPrefix(lower, "验收命令:"):
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				add(trimmed[idx+1:])
			}
		}
	}

	// Inline backtick commands after run/execute/验证
	lower := strings.ToLower(instruction)
	for _, kw := range []string{"run `", "execute `", "验证 `", "验收 `"} {
		if idx := strings.Index(lower, kw); idx >= 0 {
			rest := instruction[idx+len(kw):]
			if end := strings.Index(rest, "`"); end > 0 {
				add(rest[:end])
			}
		}
	}
	return cmds
}

func mergeVerifyCommands(plan AcceptancePlan, instruction string) AcceptancePlan {
	extra := parseVerifyCommandsFromInstruction(instruction)
	if len(extra) == 0 {
		return plan
	}
	seen := make(map[string]bool, len(plan.VerifyCommands))
	for _, c := range plan.VerifyCommands {
		seen[strings.TrimSpace(c)] = true
	}
	for _, c := range extra {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		plan.VerifyCommands = append(plan.VerifyCommands, c)
	}
	return plan
}
