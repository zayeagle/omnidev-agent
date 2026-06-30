package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestRunVerifyCommandsEmpty(t *testing.T) {
	summary, ok := RunVerifyCommands(context.Background(), "", nil)
	if !ok || summary != "" {
		t.Fatalf("expected empty pass, got ok=%v summary=%q", ok, summary)
	}
}

func TestRunVerifyCommandsEcho(t *testing.T) {
	dir := t.TempDir()
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo ok"
	} else {
		cmd = "echo ok"
	}
	summary, ok := RunVerifyCommands(context.Background(), dir, []string{cmd})
	if !ok {
		t.Fatalf("expected pass, summary=%q", summary)
	}
	if summary == "" {
		t.Fatal("expected summary text")
	}
}

func TestRunVerifyCommandsFailure(t *testing.T) {
	dir := t.TempDir()
	failCmd := "exit 1"
	if runtime.GOOS == "windows" {
		failCmd = "exit /b 1"
	}
	_, ok := RunVerifyCommands(context.Background(), dir, []string{failCmd})
	if ok {
		t.Fatal("expected failure exit code")
	}
}

func TestMergeVerifyCommandsDedupes(t *testing.T) {
	plan := AcceptancePlan{VerifyCommands: []string{"go test ./..."}}
	merged := mergeVerifyCommands(plan, "verify: go test ./...")
	if len(merged.VerifyCommands) != 1 {
		t.Fatalf("expected deduped command, got %v", merged.VerifyCommands)
	}
}

func TestRunMechanicalVerifyWithCustomCommands(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(nil, nil, nil, session.New())
	plan := AcceptancePlan{VerifyCommands: []string{"go version"}}
	mech := a.runMechanicalVerify(context.Background(), dir, plan)
	if !mech.allOK() {
		t.Fatalf("expected custom verify pass, summary=%q", mech.Summary)
	}
}
