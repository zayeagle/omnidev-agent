package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

// shellExecTool runs a shell command with a timeout.
type shellExecTool struct{}

func (t *shellExecTool) Name() string { return "shell_exec" }
func (t *shellExecTool) Description() string {
	return "Execute a shell command with a 30-second timeout. Uses cmd /C on Windows and sh -c on Linux/macOS (any CPU arch)."
}
func (t *shellExecTool) Level() permissions.Level { return permissions.LevelDangerous }
func (t *shellExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"cmd": map[string]interface{}{
			"type":        "string",
			"description": "The shell command to execute.",
			"required":    true,
		},
		"workdir": map[string]interface{}{
			"type":        "string",
			"description": "Working directory for the command. Defaults to current directory.",
		},
	}
}
func (t *shellExecTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	cmdStr := getStringArg(args, "cmd", "")
	if cmdStr == "" {
		return ErrResult("cmd is required")
	}
	workdir := getStringArg(args, "workdir", "")

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := shellCommandContext(ctx, cmdStr)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + stderr.String())
	if output == "" {
		output = "(no output)"
	}
	if err != nil {
		if ctx.Err() != nil {
			return ErrResult("command timed out after 30s")
		}
		if errors.Is(err, exec.ErrNotFound) {
			return ErrResult(fmt.Sprintf("shell not available on %s/%s: %v", runtime.GOOS, runtime.GOARCH, err))
		}
		return ErrResult("exit code: " + err.Error() + "\n" + output)
	}
	return OkResult(output)
}
