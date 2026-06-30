package tui

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

func TestPromptHistoryFromTurns(t *testing.T) {
	turns := components.NewTurnList(10)
	turns.AddTurn(1, "分析贪吃蛇")
	turns.AddTurn(2, "/help")
	turns.AddTurn(3, "修复 bug")

	got := promptHistoryFromTurns(turns)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != "分析贪吃蛇" || got[1] != "修复 bug" {
		t.Fatalf("got %#v", got)
	}
}

func TestRestoreInputHistory(t *testing.T) {
	turns := components.NewTurnList(10)
	turns.AddTurn(1, "question one")
	input := components.NewInputLine()
	restoreInputHistory(input, turns)

	input.HistPrev()
	if input.Text() != "question one" {
		t.Fatalf("restored = %q", input.Text())
	}
}
