package agent

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestDeriveProjectDir_CommonPrompts(t *testing.T) {
	cases := map[string]string{
		"完成一个贪吃蛇的小游戏":              "snake-game",
		"生成贪吃蛇":                     "snake-game",
		"implement terminal snake game": "snake-game",
		"build a simple blog app":       "blog",
		"写一个终端小游戏":                   "terminal-game",
		"帮我做一个终端 snake 游戏":          "snake-game",
		"做一个游戏":                      "game",
	}
	for prompt, want := range cases {
		got := DeriveProjectDir(prompt)
		if got != want {
			t.Errorf("DeriveProjectDir(%q) = %q, want %q", prompt, got, want)
		}
	}
}

func TestDeriveProjectDirFromTexts_PrefersNamedOverFallback(t *testing.T) {
	got := DeriveProjectDirFromTexts("继续完善", "完成一个贪吃蛇的小游戏")
	if got != "snake-game" {
		t.Fatalf("expected snake-game from earlier user text, got %s", got)
	}
}

func TestShouldReuseProjectWorkspace(t *testing.T) {
	a := &Agent{outputDir: t.TempDir()}
	a.session = session.New()
	a.session.Add(session.Entry{Role: "user", Content: "完成贪吃蛇"})
	if !a.shouldReuseProjectWorkspace("继续完善") {
		t.Fatal("expected reuse on follow-up")
	}
	if a.shouldReuseProjectWorkspace("再做一个计算器") {
		t.Fatal("expected new workspace for new project request")
	}
}
