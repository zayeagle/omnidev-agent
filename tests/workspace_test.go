package tests

import (
	"path/filepath"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func TestValidateWorkspacePath_BlocksOutside(t *testing.T) {
	a := agent.New(llm.NewMockProvider(nil), permissions.NewChecker(false), tools.NewRegistry(), session.New())
	a.SetOutputDir(t.TempDir())

	msg, ok := a.ValidateWorkspacePathForTest("write_file", map[string]interface{}{"path": "/etc/passwd"})
	if ok {
		t.Fatal("expected block outside workspace")
	}
	if msg == "" {
		t.Fatal("expected block message")
	}
}

func TestValidateWorkspacePath_AllowsInside(t *testing.T) {
	root := t.TempDir()
	a := agent.New(llm.NewMockProvider(nil), permissions.NewChecker(false), tools.NewRegistry(), session.New())
	a.SetOutputDir(root)

	msg, ok := a.ValidateWorkspacePathForTest("write_file", map[string]interface{}{"path": root + "/main.go"})
	if !ok {
		t.Fatalf("expected allow inside workspace, got block: %s", msg)
	}
}

func TestValidateLegacyWrite_BlocksRootGo(t *testing.T) {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()
	a := agent.New(llm.NewMockProvider(nil), permissions.NewChecker(false), toolbox, sess)
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.SetGuard(agent.NewProjectAwarenessGuard(toolbox, sess, repoRoot))

	msg, ok := a.ValidateLegacyWriteForTest("write_file", map[string]interface{}{"path": "hello_server.go"})
	if ok {
		t.Fatal("expected block for root-level .go in legacy repo")
	}
	if msg == "" {
		t.Fatal("expected block message")
	}
}

func TestValidateLegacyWrite_AllowsAgentSource(t *testing.T) {
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()
	a := agent.New(llm.NewMockProvider(nil), permissions.NewChecker(false), toolbox, sess)
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	a.SetGuard(agent.NewProjectAwarenessGuard(toolbox, sess, repoRoot))

	_, ok := a.ValidateLegacyWriteForTest("write_file", map[string]interface{}{"path": "internal/agent/loop.go"})
	if !ok {
		t.Fatal("expected allow edit under internal/agent/")
	}
}
