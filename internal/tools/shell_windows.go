//go:build windows

package tools

import (
	"context"
	"os"
	"os/exec"
)

func shellCommandContext(ctx context.Context, cmdLine string) *exec.Cmd {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	return exec.CommandContext(ctx, comspec, "/C", cmdLine)
}
