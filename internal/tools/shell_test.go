package tools

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestShellExec_Echo(t *testing.T) {
	tool := &shellExecTool{}
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo omnidev-shell-ok"
	} else {
		cmd = "echo omnidev-shell-ok"
	}

	res := tool.Execute(context.Background(), map[string]interface{}{"cmd": cmd})
	if !res.Success {
		t.Fatalf("shell_exec failed on %s/%s: %s", runtime.GOOS, runtime.GOARCH, res.Error)
	}
	if !strings.Contains(res.Data, "omnidev-shell-ok") {
		t.Fatalf("unexpected output on %s/%s: %q", runtime.GOOS, runtime.GOARCH, res.Data)
	}
}

func TestShellExec_Workdir(t *testing.T) {
	tool := &shellExecTool{}
	dir := t.TempDir()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd"
	} else {
		cmd = "pwd"
	}

	res := tool.Execute(context.Background(), map[string]interface{}{
		"cmd":     cmd,
		"workdir": dir,
	})
	if !res.Success {
		t.Fatalf("shell_exec workdir failed: %s", res.Error)
	}
	if !strings.Contains(strings.ToLower(res.Data), strings.ToLower(dir)) {
		t.Fatalf("expected workdir %q in output %q", dir, res.Data)
	}
}

func TestShellExec_EmptyCmd(t *testing.T) {
	tool := &shellExecTool{}
	res := tool.Execute(context.Background(), map[string]interface{}{"cmd": ""})
	if res.Success {
		t.Fatal("expected error for empty cmd")
	}
}
