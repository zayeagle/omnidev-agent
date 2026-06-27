//go:build !windows

package tools

import (
	"context"
	"os"
	"os/exec"
)

func shellCommandContext(ctx context.Context, cmdLine string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.CommandContext(ctx, shell, "-c", cmdLine)
}
