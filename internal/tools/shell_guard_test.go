package tools

import "testing"

func TestLongRunningShellReason_BlocksGoRun(t *testing.T) {
	cmds := []string{
		"go run .",
		"cd foo && start /B go run ./...",
		"GO RUN main.go",
		"npm run dev",
		"yarn start",
	}
	for _, cmd := range cmds {
		if reason := LongRunningShellReason(cmd); reason == "" {
			t.Fatalf("expected block for %q", cmd)
		}
	}
}

func TestLongRunningShellReason_AllowsBuild(t *testing.T) {
	cmds := []string{
		"go build ./...",
		"go test ./...",
		"echo hello",
	}
	for _, cmd := range cmds {
		if reason := LongRunningShellReason(cmd); reason != "" {
			t.Fatalf("unexpected block for %q: %s", cmd, reason)
		}
	}
}

func TestShellExec_BlocksGoRun(t *testing.T) {
	tool := &shellExecTool{}
	res := tool.Execute(t.Context(), map[string]interface{}{"cmd": "go run ."})
	if res.Success {
		t.Fatal("expected go run to be blocked")
	}
	if res.Error == "" {
		t.Fatal("expected error message")
	}
}
