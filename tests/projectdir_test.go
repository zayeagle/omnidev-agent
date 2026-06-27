package tests

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/agent"
)

func TestDeriveProjectDir_SnakeGame(t *testing.T) {
	got := agent.DeriveProjectDir("完成一个贪吃蛇的小游戏")
	if got != "snake-game" {
		t.Errorf("expected snake-game, got %s", got)
	}
}

func TestDeriveProjectDir_English(t *testing.T) {
	got := agent.DeriveProjectDir("build a simple blog app")
	if got != "blog" {
		t.Errorf("expected blog, got %s", got)
	}
}

func TestEnsureProjectWorkspace(t *testing.T) {
	dir := t.TempDir()
	path, err := agent.EnsureProjectWorkspace(dir, "贪吃蛇游戏")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestIsNewProjectRequest_Calculator(t *testing.T) {
	if !agent.IsNewProjectRequest("implement a simple calculator") {
		t.Error("expected calculator request to be new project")
	}
}

func TestIsNewProjectRequest_LegacyEdit(t *testing.T) {
	if agent.IsNewProjectRequest("fix the bug in internal/agent/loop.go") {
		t.Error("expected legacy edit not to be new project")
	}
}
